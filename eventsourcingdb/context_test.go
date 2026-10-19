package eventsourcingdb_test

import (
	"context"
	"net/http"
	"net/http/httptrace"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
)

// A caller has to be able to end every request with its context, e.g. a write
// that the server does not answer, and to tell a read that was aborted from
// one that is complete. Otherwise whatever it decides on the events read so
// far looks as if it had seen all of them. It also has to be able to learn
// about a request from the values of its context, e.g. from a trace whether a
// write that failed was sent at all.
func TestTheContext(t *testing.T) {
	t.Run("a read that is canceled midway ends with the error of the context", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
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

		_, err = client.WriteEvents(ctx, []eventsourcingdb.EventCandidate{candidate, candidate, candidate}, nil)
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

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
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

	t.Run("a write with a context that has already ended writes nothing", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		ended, cancel := context.WithCancel(ctx)
		cancel()

		_, err = client.WriteEvents(ended, []eventsourcingdb.EventCandidate{
			{
				Source:  "https://www.eventsourcingdb.io",
				Subject: "/test",
				Type:    "io.eventsourcingdb.test",
				Data:    map[string]int{"value": 23},
			},
		}, nil)
		assert.ErrorIs(t, err, context.Canceled)

		eventsRead := 0
		for _, err := range client.ReadEvents(ctx, "/", eventsourcingdb.ReadEventsOptions{Recursive: true}) {
			require.NoError(t, err)
			eventsRead++
		}

		assert.Zero(t, eventsRead)
	})

	for _, api := range apis() {
		t.Run(api.name, func(t *testing.T) {
			t.Run("a deadline ends a request that the server does not answer", func(t *testing.T) {
				client := clientOfAServerThatHangs(t, api, nil)

				// Every function that takes a context has to hand it on to its
				// request, or the deadline does not end the request, and for a
				// stream only takes effect between two lines that never arrive.
				for name, request := range requestsWithContextOf(client) {
					t.Run(name, func(t *testing.T) {
						ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
						defer cancel()

						started := time.Now()
						err := request(ctx)

						assert.ErrorIs(t, err, context.DeadlineExceeded)
						assert.Less(t, time.Since(started), 2*time.Second, "the deadline did not end the request")
					})
				}
			})

			t.Run("a cancel ends a request that the server does not answer", func(t *testing.T) {
				client := clientOfAServerThatHangs(t, api, nil)

				for name, request := range requestsWithContextOf(client) {
					t.Run(name, func(t *testing.T) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()

						go func() {
							time.Sleep(100 * time.Millisecond)
							cancel()
						}()

						started := time.Now()
						err := request(ctx)

						assert.ErrorIs(t, err, context.Canceled)
						assert.Less(t, time.Since(started), 2*time.Second, "the cancel did not end the request")
					})
				}
			})

			t.Run("a request with a context that has already ended is not sent", func(t *testing.T) {
				var requestsReceived atomic.Int32
				client := clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestsReceived.Add(1)
					w.Header().Set("Server", "EventSourcingDB/test")
					w.WriteHeader(http.StatusOK)
				}))

				for name, request := range requestsWithContextOf(client) {
					t.Run(name, func(t *testing.T) {
						ended, cancel := context.WithCancel(context.Background())
						cancel()

						requestsBefore := requestsReceived.Load()
						err := request(ended)

						assert.ErrorIs(t, err, context.Canceled)
						assert.Equal(t, requestsBefore, requestsReceived.Load(), "the request was sent")
					})
				}
			})

			t.Run("a deadline ends a write whose response stops in the middle", func(t *testing.T) {
				start := `[{"specversion":"1.0",`
				if api.name == "v2" {
					start = api.heartbeat() + "\n" + `{"type":"event","data":{"specversion":"1.0",`
				}
				client := clientOfAServerThatHangs(t, api, []byte(start))

				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				started := time.Now()
				_, err := client.WriteEvents(ctx, []eventsourcingdb.EventCandidate{
					{
						Source:  "https://www.eventsourcingdb.io",
						Subject: "/test",
						Type:    "io.eventsourcingdb.test",
						Data:    map[string]int{"value": 23},
					},
				}, nil)

				assert.ErrorIs(t, err, context.DeadlineExceeded)
				assert.Less(t, time.Since(started), 2*time.Second, "the deadline did not end the response")
			})

			t.Run("a request carries the values of its context, such as a trace", func(t *testing.T) {
				client := clientOfAServerThatAnswers(t, api, http.StatusServiceUnavailable, "")

				// A caller learns from a trace in the context whether a request was
				// written to the connection, e.g. to tell whether a write that failed
				// may have reached the server. That only works if every function
				// hands on the context it was given, or one derived from it, and not
				// one that drops its values.
				for name, request := range requestsWithContextOf(client) {
					t.Run(name, func(t *testing.T) {
						wroteRequest := make(chan httptrace.WroteRequestInfo, 1)
						ctx := httptrace.WithClientTrace(context.Background(), &httptrace.ClientTrace{
							WroteRequest: func(info httptrace.WroteRequestInfo) {
								select {
								case wroteRequest <- info:
								default:
								}
							},
						})

						_ = request(ctx)

						select {
						case info := <-wroteRequest:
							assert.NoError(t, info.Err)
						case <-time.After(time.Second):
							assert.Fail(t, "the trace of the context did not reach the request")
						}
					})
				}
			})

			t.Run("a cancel ends a stream that stops in the middle", func(t *testing.T) {
				firstLine := streams()[0].item(api) + "\n"
				client := clientOfAServerThatHangs(t, api, []byte(firstLine))

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
				firstLine := streams()[0].item(api) + "\n"
				client := clientOfAServerThatHangs(t, api, []byte(firstLine))

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
		})
	}
}
