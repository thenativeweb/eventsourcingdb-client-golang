package eventsourcingdb_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// A caller has to learn when a request stalls, e.g. because the server can
// not be reached, or behind a proxy that keeps the connection open but no
// longer passes anything on. Otherwise it waits forever. A request that keeps
// receiving something, however long it takes, must not end, though.
func TestErrConnectionStalled(t *testing.T) {
	timeout := 250 * time.Millisecond
	withTimeout := eventsourcingdb.WithStallTimeout(timeout)

	t.Run("every request ends if the server does not answer", func(t *testing.T) {
		client := clientOfAServerThatHangs(t, nil, withTimeout)

		for name, request := range requestsWithContextOf(client) {
			t.Run(name, func(t *testing.T) {
				started := time.Now()
				err := request(context.Background())
				elapsed := time.Since(started)

				// Before the server answers, a stream with heartbeats stalls
				// as every other request does.
				assert.ErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
				assert.NotErrorIs(t, err, eventsourcingdb.ErrHeartbeatTimeout)
				assert.EqualError(t, err, "connection stalled, nothing arrived from the server for 250ms")
				assert.GreaterOrEqual(t, elapsed, timeout)
				assert.Less(t, elapsed, 2*time.Second, "the stall timeout did not end the request")
			})
		}
	})

	t.Run("every request ends if the response stops in the middle", func(t *testing.T) {
		client := clientOfAServerThatHangs(t, []byte(`{"type":`), withTimeout)

		for name, request := range requestsWithContextOf(client) {
			// Registering an event schema does not read the body of a
			// response with status 200, so it does not wait for it.
			if name == "RegisterEventSchema" {
				continue
			}

			t.Run(name, func(t *testing.T) {
				started := time.Now()
				err := request(context.Background())
				elapsed := time.Since(started)

				// Once the server answers, a stream with heartbeats ends with
				// ErrHeartbeatTimeout, as before.
				if name == "ObserveEvents" || name == "RunEventQLQuery" {
					assert.ErrorIs(t, err, eventsourcingdb.ErrHeartbeatTimeout)
					assert.NotErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
				} else {
					assert.ErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
				}
				assert.GreaterOrEqual(t, elapsed, timeout)
				assert.Less(t, elapsed, 2*time.Second, "the stall timeout did not end the request")
			})
		}
	})

	t.Run("a request ends if the reason of an unexpected status stops in the middle", func(t *testing.T) {
		client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "EventSourcingDB/test")
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("service"))
			w.(http.Flusher).Flush()

			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		}), withTimeout)

		err := client.Ping(context.Background())

		var dbAPIError *eventsourcingdb.DBAPIError
		require.ErrorAs(t, err, &dbAPIError)
		assert.Equal(t, http.StatusServiceUnavailable, dbAPIError.StatusCode)
		assert.ErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
		assert.EqualError(t, err, "failed to ping, got HTTP status code '503', expected '200', and failed to read the reason: connection stalled, nothing arrived from the server for 250ms")
	})

	t.Run("every byte keeps a request alive", func(t *testing.T) {
		// Before the header and between the parts of the body, the server
		// waits for half the stall timeout, so the response takes more than
		// twice as long as the stall timeout.
		client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(timeout / 2)
			w.Header().Set("Server", "EventSourcingDB/test")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()

			for _, part := range []string{`{"type":`, `"io.eventsourcingdb.api.`, `ping-received"`, `}`} {
				time.Sleep(timeout / 2)
				_, _ = w.Write([]byte(part))
				w.(http.Flusher).Flush()
			}
		}), withTimeout)

		started := time.Now()
		err := client.Ping(context.Background())

		assert.NoError(t, err)
		assert.Greater(t, time.Since(started), 2*timeout)
	})

	t.Run("items keep every stream alive", func(t *testing.T) {
		for _, stream := range streams() {
			t.Run(stream.name, func(t *testing.T) {
				// The items last for twice as long as the stall timeout.
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					for range 4 {
						send(stream.line)
						time.Sleep(timeout / 2)
					}
				}, withTimeout)

				items, err := stream.read(context.Background(), client, nil)

				assert.NoError(t, err)
				assert.Equal(t, 4, items)
			})
		}
	})

	t.Run("a stream ends if nothing arrives after an item", func(t *testing.T) {
		for _, stream := range streamsWithoutHeartbeats() {
			t.Run(stream.name, func(t *testing.T) {
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					send(stream.line)

					select {
					case <-ctx.Done():
					case <-time.After(5 * time.Second):
					}
				}, withTimeout)

				started := time.Now()
				items, err := stream.read(context.Background(), client, nil)
				elapsed := time.Since(started)

				assert.Equal(t, 1, items)
				assert.ErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
				assert.Less(t, elapsed, 2*time.Second, "the stall timeout did not end the stream")
			})
		}
	})

	t.Run("the time the caller spends on an item does not count", func(t *testing.T) {
		for _, stream := range streams() {
			t.Run(stream.name, func(t *testing.T) {
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					send(stream.line)
					send(stream.line)
				}, withTimeout)

				items, err := stream.read(context.Background(), client, func() {
					time.Sleep(2 * timeout)
				})

				assert.NoError(t, err)
				assert.Equal(t, 2, items)
			})
		}
	})

	t.Run("a deadline of the context still ends a request earlier", func(t *testing.T) {
		client := clientOfAServerThatHangs(t, nil, eventsourcingdb.WithStallTimeout(time.Minute))

		for name, request := range requestsWithContextOf(client) {
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				started := time.Now()
				err := request(ctx)

				assert.ErrorIs(t, err, context.DeadlineExceeded)
				assert.NotErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
				assert.Less(t, time.Since(started), 2*time.Second, "the deadline did not end the request")
			})
		}
	})

	t.Run("a cancel ends a request as before, not as a stall", func(t *testing.T) {
		client := clientOfAServerThatHangs(t, nil, withTimeout)

		for name, request := range requestsWithContextOf(client) {
			t.Run(name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				go func() {
					time.Sleep(timeout / 5)
					cancel()
				}()

				err := request(ctx)

				assert.ErrorIs(t, err, context.Canceled)
				assert.NotErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
			})
		}
	})

	t.Run("is found if it is wrapped, and counts as a timeout", func(t *testing.T) {
		client := clientOfAServerThatHangs(t, nil, withTimeout)

		err := client.Ping(context.Background())
		wrapped := fmt.Errorf("failed to handle the command: %w", err)

		assert.ErrorIs(t, wrapped, eventsourcingdb.ErrConnectionStalled)
		assert.ErrorIs(t, wrapped, context.DeadlineExceeded)

		var timeoutError interface{ Timeout() bool }
		require.True(t, errors.As(wrapped, &timeoutError))
		assert.True(t, timeoutError.Timeout())
	})
}

func TestWithStallTimeout(t *testing.T) {
	baseURL, err := url.Parse("http://localhost:3000")
	require.NoError(t, err)

	t.Run("is 30 seconds by default", func(t *testing.T) {
		client, err := eventsourcingdb.NewClient(baseURL, "secret")
		require.NoError(t, err)

		assert.Equal(t, 30*time.Second, eventsourcingdb.StallTimeoutOf(client))
	})

	t.Run("sets the stall timeout", func(t *testing.T) {
		client, err := eventsourcingdb.NewClient(baseURL, "secret", eventsourcingdb.WithStallTimeout(5*time.Second))
		require.NoError(t, err)

		assert.Equal(t, 5*time.Second, eventsourcingdb.StallTimeoutOf(client))
	})

	t.Run("must be greater than zero", func(t *testing.T) {
		for _, stallTimeout := range []time.Duration{0, -time.Second} {
			client, err := eventsourcingdb.NewClient(baseURL, "secret", eventsourcingdb.WithStallTimeout(stallTimeout))

			assert.Nil(t, client)
			assert.EqualError(t, err, "stall timeout must be greater than zero")
		}
	})

	t.Run("is the only bound of a request", func(t *testing.T) {
		// A Timeout of the HTTP client would end every stream after it,
		// however much it receives, and the 10 seconds that the default
		// transport gives the TLS handshake would end it before a longer
		// stall timeout.
		client, err := eventsourcingdb.NewClient(baseURL, "secret", eventsourcingdb.WithStallTimeout(time.Minute))
		require.NoError(t, err)

		httpClient := eventsourcingdb.HTTPClientOf(client)
		assert.Zero(t, httpClient.Timeout)

		transport, ok := httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		assert.Equal(t, time.Minute, transport.TLSHandshakeTimeout)
		assert.Zero(t, transport.ResponseHeaderTimeout)
		assert.NotSame(t, http.DefaultTransport, transport)
	})
}

// streams returns every function of the client that reads a stream, with and
// without heartbeats.
func streams() []streamWithHeartbeats {
	return append(streamsWithHeartbeats(), streamsWithoutHeartbeats()...)
}

// streamsWithoutHeartbeats returns every function of the client that reads a
// stream on which the server sends no heartbeats, as streamsWithHeartbeats
// does for those with heartbeats.
func streamsWithoutHeartbeats() []streamWithHeartbeats {
	return []streamWithHeartbeats{
		{
			name: "ReadEvents",
			line: `{"type":"event","payload":{"specversion":"1.0","id":"0","time":"2026-10-01T12:00:00Z","source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","datacontenttype":"application/json","data":{},"hash":"","predecessorhash":""}}`,
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				items := 0
				for _, err := range client.ReadEvents(ctx, "/test", eventsourcingdb.ReadEventsOptions{}) {
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
			name: "ReadSubjects",
			line: `{"type":"subject","payload":{"subject":"/test"}}`,
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				items := 0
				for _, err := range client.ReadSubjects(ctx, "/") {
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
			name: "ReadEventTypes",
			line: `{"type":"eventType","payload":{"eventType":"io.eventsourcingdb.test","isPhantom":false}}`,
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				items := 0
				for _, err := range client.ReadEventTypes(ctx) {
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
