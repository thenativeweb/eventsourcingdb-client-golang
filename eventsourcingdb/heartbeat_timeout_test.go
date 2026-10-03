package eventsourcingdb_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
)

const heartbeatLine = `{"type":"heartbeat","payload":{}}`

// A caller that observes events has to learn when the connection stalls, e.g.
// behind a proxy that keeps it open but no longer passes anything on.
// Otherwise it keeps believing it is live, while nothing arrives any more.
func TestErrHeartbeatTimeout(t *testing.T) {
	timeout := 250 * time.Millisecond
	eventsourcingdb.SetHeartbeatTimeout(t, timeout)

	t.Run("every stream with heartbeats ends if nothing arrives after a heartbeat", func(t *testing.T) {
		for _, stream := range streamsWithHeartbeats() {
			t.Run(stream.name, func(t *testing.T) {
				connectionClosed := make(chan struct{})
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					send(heartbeatLine)

					select {
					case <-ctx.Done():
						close(connectionClosed)
					case <-time.After(5 * time.Second):
					}
				})

				started := time.Now()
				_, err := stream.read(context.Background(), client, nil)
				elapsed := time.Since(started)

				assert.ErrorIs(t, err, eventsourcingdb.ErrHeartbeatTimeout)
				assert.EqualError(t, err, "no event and no heartbeat arrived for 30 seconds")
				assert.GreaterOrEqual(t, elapsed, timeout)
				assert.Less(t, elapsed, 2*time.Second, "the heartbeat timeout did not end the stream")

				select {
				case <-connectionClosed:
				case <-time.After(time.Second):
					assert.Fail(t, "the client did not close the connection")
				}
			})
		}
	})

	t.Run("heartbeats keep a stream alive", func(t *testing.T) {
		for _, stream := range streamsWithHeartbeats() {
			t.Run(stream.name, func(t *testing.T) {
				// The heartbeats last for three times as long as the timeout.
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					for range 15 {
						send(heartbeatLine)
						time.Sleep(timeout / 5)
					}
					send(stream.line)
				})

				items, err := stream.read(context.Background(), client, nil)

				assert.NoError(t, err)
				assert.Equal(t, 1, items)
			})
		}
	})

	t.Run("items that arrive within the timeout are handed out as before", func(t *testing.T) {
		for _, stream := range streamsWithHeartbeats() {
			t.Run(stream.name, func(t *testing.T) {
				// Without heartbeats in between, the items last for twice as
				// long as the timeout.
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					for range 4 {
						send(stream.line)
						time.Sleep(timeout / 2)
					}
				})

				items, err := stream.read(context.Background(), client, nil)

				assert.NoError(t, err)
				assert.Equal(t, 4, items)
			})
		}
	})

	t.Run("the time the caller spends on an item does not count", func(t *testing.T) {
		for _, stream := range streamsWithHeartbeats() {
			t.Run(stream.name, func(t *testing.T) {
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					send(stream.line)
					send(stream.line)
				})

				items, err := stream.read(context.Background(), client, func() {
					time.Sleep(2 * timeout)
				})

				assert.NoError(t, err)
				assert.Equal(t, 2, items)
			})
		}
	})

	t.Run("a cancel ends a stream as before, not with a heartbeat timeout", func(t *testing.T) {
		for _, stream := range streamsWithHeartbeats() {
			t.Run(stream.name, func(t *testing.T) {
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					send(heartbeatLine)

					select {
					case <-ctx.Done():
					case <-time.After(5 * time.Second):
					}
				})

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				go func() {
					time.Sleep(timeout / 5)
					cancel()
				}()

				_, err := stream.read(ctx, client, nil)

				assert.ErrorIs(t, err, context.Canceled)
				assert.NotErrorIs(t, err, eventsourcingdb.ErrHeartbeatTimeout)
			})
		}
	})

	t.Run("is found if it is wrapped", func(t *testing.T) {
		client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
			send(heartbeatLine)

			select {
			case <-ctx.Done():
			case <-time.After(5 * time.Second):
			}
		})

		var err error
		for _, observeErr := range client.ObserveEvents(context.Background(), "/test", eventsourcingdb.ObserveEventsOptions{}) {
			if observeErr != nil {
				err = observeErr
				break
			}
		}
		wrapped := fmt.Errorf("failed to handle the command: %w", err)

		assert.ErrorIs(t, wrapped, eventsourcingdb.ErrHeartbeatTimeout)
	})
}

// clientOfAServerThatStreams returns a client of a server that answers every
// request as EventSourcingDB would, and then sends every line that stream
// hands to send, right away. The response ends when stream returns. The
// context that stream gets ends when the client closes the connection, and
// every stream that waits for it also has to give up after a few seconds, so
// that a client that does not close the connection still ends.
func clientOfAServerThatStreams(t *testing.T, stream func(ctx context.Context, send func(line string))) *eventsourcingdb.Client {
	t.Helper()

	return clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "EventSourcingDB/test")
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()

		stream(r.Context(), func(line string) {
			_, _ = w.Write([]byte(line + "\n"))
			w.(http.Flusher).Flush()
		})
	}))
}

type streamWithHeartbeats struct {
	name string
	line string
	read func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error)
}

// streamsWithHeartbeats returns every function of the client that reads a
// stream on which the server sends heartbeats, together with a line that
// carries one item of that stream. read reads until the stream ends, calls
// handle for every item if it is given, and returns how many items there were
// and the error the stream ends with.
func streamsWithHeartbeats() []streamWithHeartbeats {
	return []streamWithHeartbeats{
		{
			name: "ObserveEvents",
			line: `{"type":"event","payload":{"specversion":"1.0","id":"0","time":"2026-10-01T12:00:00Z","source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","datacontenttype":"application/json","data":{},"hash":"","predecessorhash":""}}`,
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				items := 0
				for _, err := range client.ObserveEvents(ctx, "/test", eventsourcingdb.ObserveEventsOptions{}) {
					if err != nil {
						return items, err
					}

					items++
					if handle != nil {
						handle()
					}
				}
				return items, nil
			},
		},
		{
			name: "RunEventQLQuery",
			line: `{"type":"row","payload":{"value":23}}`,
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				items := 0
				for _, err := range client.RunEventQLQuery(ctx, "FROM e IN events PROJECT INTO e") {
					if err != nil {
						return items, err
					}

					items++
					if handle != nil {
						handle()
					}
				}
				return items, nil
			},
		},
	}
}
