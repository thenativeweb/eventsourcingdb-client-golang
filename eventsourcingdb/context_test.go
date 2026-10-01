package eventsourcingdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

// A caller has to be able to tell a read that was aborted from one that is
// complete. Otherwise whatever it decides on the events read so far looks as
// if it had seen all of them.
func TestTheContext(t *testing.T) {
	t.Run("a read that is canceled midway ends with the error of the context", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		candidate := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data:    map[string]int{"value": 23},
		}

		_, err = client.WriteEvents([]eventsourcingdb.EventCandidate{candidate, candidate, candidate}, nil)
		require.NoError(t, err)

		readCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		eventsRead := 0
		var readErr error

		for _, err := range client.ReadEvents(readCtx, "/test", eventsourcingdb.ReadEventsOptions{}) {
			if err != nil {
				readErr = err
				break
			}

			eventsRead++
			cancel()
		}

		assert.Equal(t, 1, eventsRead)
		assert.ErrorIs(t, readErr, context.Canceled)
	})

	t.Run("a read with a context that has already ended does not start", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		ended, cancel := context.WithCancel(ctx)
		cancel()

		var readErr error
		for _, err := range client.ReadEvents(ended, "/test", eventsourcingdb.ReadEventsOptions{}) {
			readErr = err
			break
		}

		assert.ErrorIs(t, readErr, context.Canceled)
	})

	t.Run("a deadline ends a request that the server does not answer", func(t *testing.T) {
		client := clientOfAServerThatHangs(t, nil)

		// Every function that reads with a context has to hand it on to the
		// request, or the deadline only takes effect between two lines of a
		// stream that never arrives.
		for name, read := range readsOf(client) {
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				started := time.Now()
				err := read(ctx)

				assert.ErrorIs(t, err, context.DeadlineExceeded)
				assert.Less(t, time.Since(started), 2*time.Second, "the deadline did not end the request")
			})
		}
	})

	t.Run("a cancel ends a stream that stops in the middle", func(t *testing.T) {
		firstLine := `{"type":"event","payload":{"specversion":"1.0","id":"0","time":"2026-10-01T12:00:00Z","source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","datacontenttype":"application/json","data":{},"hash":"","predecessorhash":""}}` + "\n"
		client := clientOfAServerThatHangs(t, []byte(firstLine))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		var readErr error
		for _, err := range client.ReadEvents(ctx, "/test", eventsourcingdb.ReadEventsOptions{}) {
			if err != nil {
				readErr = err
				break
			}
		}

		assert.ErrorIs(t, readErr, context.Canceled)
	})

	t.Run("a deadline ends a stream that stops in the middle", func(t *testing.T) {
		firstLine := `{"type":"event","payload":{"specversion":"1.0","id":"0","time":"2026-10-01T12:00:00Z","source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","datacontenttype":"application/json","data":{},"hash":"","predecessorhash":""}}` + "\n"
		client := clientOfAServerThatHangs(t, []byte(firstLine))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		started := time.Now()
		eventsRead := 0
		var readErr error

		for _, err := range client.ReadEvents(ctx, "/test", eventsourcingdb.ReadEventsOptions{}) {
			if err != nil {
				readErr = err
				break
			}

			eventsRead++
		}

		assert.Equal(t, 1, eventsRead)
		assert.ErrorIs(t, readErr, context.DeadlineExceeded)
		assert.Less(t, time.Since(started), 2*time.Second, "the deadline did not end the stream")
	})
}

// clientOfAServerThatHangs returns a client of a server that answers as
// EventSourcingDB would, but stops: without a start of a stream, before it
// answers at all, and with one, after sending it. It waits until the request
// ends, until the test is over, or until five seconds have passed, so that a
// client that ignores its context still ends.
func clientOfAServerThatHangs(t *testing.T, start []byte) *eventsourcingdb.Client {
	t.Helper()

	released := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if start != nil {
			w.Header().Set("Server", "EventSourcingDB/test")
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(start)
			w.(http.Flusher).Flush()
		}

		select {
		case <-r.Context().Done():
		case <-released:
		case <-time.After(5 * time.Second):
		}
	}))
	t.Cleanup(func() {
		close(released)
		server.Close()
	})

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	client, err := eventsourcingdb.NewClient(baseURL, "secret")
	require.NoError(t, err)

	return client
}

// readsOf calls every function of the client that reads with a context, and
// returns the first error each of them hands out.
func readsOf(client *eventsourcingdb.Client) map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"ReadEvents": func(ctx context.Context) error {
			for _, err := range client.ReadEvents(ctx, "/test", eventsourcingdb.ReadEventsOptions{}) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"ObserveEvents": func(ctx context.Context) error {
			for _, err := range client.ObserveEvents(ctx, "/test", eventsourcingdb.ObserveEventsOptions{}) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"ReadSubjects": func(ctx context.Context) error {
			for _, err := range client.ReadSubjects(ctx, "/") {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"ReadEventTypes": func(ctx context.Context) error {
			for _, err := range client.ReadEventTypes(ctx) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"RunEventQLQuery": func(ctx context.Context) error {
			for _, err := range client.RunEventQLQuery(ctx, "FROM e IN events PROJECT INTO e") {
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}
