package eventsourcingdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// The client has to speak the v2 API with a server that offers it, and the v1
// API with every other one, so that the same code runs against every version
// of EventSourcingDB. It asks the server on the first request, and remembers
// the answer, but only one that tells for sure.
func TestChoosingTheAPI(t *testing.T) {
	t.Run("creating a client sends no request", func(t *testing.T) {
		server := newProbedServer(t, answerAsV2)

		_, err := eventsourcingdb.NewClient(server.baseURL, "secret")
		require.NoError(t, err)

		assert.Empty(t, server.requests())
	})

	t.Run("asks the server on the first request, without the API token", func(t *testing.T) {
		server := newProbedServer(t, answerAsV2)
		client := server.client(t)

		err := client.VerifyAPIToken(context.Background())
		require.NoError(t, err)

		requests := server.requests()
		require.Len(t, requests, 2)
		assert.Equal(t, "GET /api/v2/system/ping", requests[0].String())
		assert.Empty(t, requests[0].authorization)
		assert.Equal(t, "POST /api/v2/system/verify-token", requests[1].String())
		assert.Equal(t, "Bearer secret", requests[1].authorization)
	})

	t.Run("speaks the API that the server offers, and remembers it", func(t *testing.T) {
		answers := []struct {
			name    string
			answer  probeAnswer
			api     string
			pingURL string
		}{
			{name: "the event of a ping", answer: answerAsV2, api: "v2", pingURL: "GET /api/v2/system/ping"},
			{name: "status 404", answer: answerAsV1, api: "v1", pingURL: "GET /api/v1/ping"},
			{name: "status 405", answer: answerWithStatus(http.StatusMethodNotAllowed), api: "v1", pingURL: "GET /api/v1/ping"},
			{name: "status 404 without a body", answer: answerWithStatus(http.StatusNotFound), api: "v1", pingURL: "GET /api/v1/ping"},

			// A server with the v1 API only behind a proxy that passes on
			// nothing but the paths of the v1 API, and answers every other
			// path itself.
			{name: "status 404, but not from EventSourcingDB", answer: answerFromProxy(http.StatusNotFound), api: "v1", pingURL: "GET /api/v1/ping"},
			{name: "status 405, but not from EventSourcingDB", answer: answerFromProxy(http.StatusMethodNotAllowed), api: "v1", pingURL: "GET /api/v1/ping"},
		}

		for _, answer := range answers {
			t.Run(answer.name, func(t *testing.T) {
				server := newProbedServer(t, answer.answer)
				client := server.client(t)

				for range 3 {
					err := client.Ping(context.Background())
					require.NoError(t, err)
				}

				assert.Equal(t, answer.api, eventsourcingdb.APIOf(client))
				assert.Equal(t, int32(1), server.probes.Load())
				assert.Equal(t, []string{"GET /api/v2/system/ping", answer.pingURL, answer.pingURL, answer.pingURL}, server.paths())
			})
		}
	})

	t.Run("remembers nothing without an answer that tells for sure, and asks again on the next request", func(t *testing.T) {
		answers := []struct {
			name   string
			answer probeAnswer
			check  func(t *testing.T, err error)
		}{
			{
				name:   "status 500",
				answer: answerWithStatus(http.StatusInternalServerError),
				check: func(t *testing.T, err error) {
					assertDBAPIError(t, err, http.StatusInternalServerError, "", "failed")
					assert.EqualError(t, err, "failed to read events, got HTTP status code '500', expected '200': failed")
				},
			},
			{
				name:   "status 503",
				answer: answerWithStatus(http.StatusServiceUnavailable),
				check: func(t *testing.T, err error) {
					assertDBAPIError(t, err, http.StatusServiceUnavailable, "", "failed")
				},
			},
			{
				name: "status 429",
				answer: func(w http.ResponseWriter, r *http.Request) bool {
					w.Header().Set("Retry-After", "2")
					return answerWithStatus(http.StatusTooManyRequests)(w, r)
				},
				check: func(t *testing.T, err error) {
					var dbAPIError *eventsourcingdb.DBAPIError
					require.ErrorAs(t, err, &dbAPIError)
					assert.Equal(t, http.StatusTooManyRequests, dbAPIError.Status)
					assert.Equal(t, 2*time.Second, dbAPIError.RetryAfter)
				},
			},
			{
				name: "status 200 with another event",
				answer: func(w http.ResponseWriter, r *http.Request) bool {
					w.Header().Set("Server", "EventSourcingDB/test")
					_, _ = w.Write([]byte(`{"type":"event","data":{"type":"io.eventsourcingdb.api.token-verified"}}`))
					return false
				},
				check: assertUnexpectedProbeAnswer,
			},
			{
				name: "status 200 with the event of a ping of the v1 API",
				answer: func(w http.ResponseWriter, r *http.Request) bool {
					w.Header().Set("Server", "EventSourcingDB/test")
					_, _ = w.Write([]byte(apiV1.pingAnswer()))
					return false
				},
				check: assertUnexpectedProbeAnswer,
			},
			{
				name: "status 200 with something other than JSON",
				answer: func(w http.ResponseWriter, r *http.Request) bool {
					w.Header().Set("Server", "EventSourcingDB/test")
					_, _ = w.Write([]byte(`<html></html>`))
					return false
				},
				check: assertUnexpectedProbeAnswer,
			},
			{
				name: "the event of a ping, but not from EventSourcingDB",
				answer: func(w http.ResponseWriter, r *http.Request) bool {
					_, _ = w.Write([]byte(apiV2.pingAnswer()))
					return false
				},
				check: func(t *testing.T, err error) {
					assert.ErrorIs(t, err, eventsourcingdb.ErrInvalidServerHeader)
				},
			},
			{
				name:   "status 502, but not from EventSourcingDB",
				answer: answerFromProxy(http.StatusBadGateway),
				check: func(t *testing.T, err error) {
					assert.ErrorIs(t, err, eventsourcingdb.ErrInvalidServerHeader)
					assert.EqualError(t, err, "server must be EventSourcingDB")
				},
			},
			{
				name: "a connection that breaks",
				answer: func(w http.ResponseWriter, r *http.Request) bool {
					connection, _, err := http.NewResponseController(w).Hijack()
					if err == nil {
						_ = connection.Close()
					}
					return false
				},
				check: func(t *testing.T, err error) {
					assert.Error(t, err)
					assert.NotErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
				},
			},
			{
				name: "no answer at all",
				answer: func(w http.ResponseWriter, r *http.Request) bool {
					select {
					case <-r.Context().Done():
					case <-time.After(5 * time.Second):
					}
					return false
				},
				check: func(t *testing.T, err error) {
					assert.ErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
				},
			},
		}

		for _, answer := range answers {
			t.Run(answer.name, func(t *testing.T) {
				server := newProbedServer(t, answer.answer)
				client := server.client(t, eventsourcingdb.WithStallTimeout(250*time.Millisecond))

				_, err := countItems(client.ReadEvents(context.Background(), "/test", eventsourcingdb.ReadEventsOptions{}), nil)
				answer.check(t, err)
				assert.Empty(t, eventsourcingdb.APIOf(client))
				assert.Equal(t, []string{"GET /api/v2/system/ping"}, server.paths())

				server.answerProbe(answerAsV2)

				_, err = countItems(client.ReadEvents(context.Background(), "/test", eventsourcingdb.ReadEventsOptions{}), nil)
				require.NoError(t, err)
				assert.Equal(t, "v2", eventsourcingdb.APIOf(client))
				assert.Equal(t, int32(2), server.probes.Load())
			})
		}
	})

	t.Run("asks once for requests that start while it asks", func(t *testing.T) {
		const requests = 5

		released, release := newRelease()
		server := newProbedServer(t, func(w http.ResponseWriter, r *http.Request) bool {
			select {
			case <-released:
			case <-r.Context().Done():
				return false
			}
			return answerAsV1(w, r)
		})
		client := server.client(t)
		// The server waits for its handlers when it closes, so they have to be
		// released first, also if the test fails.
		t.Cleanup(release)

		waiting := make(chan struct{}, requests)
		eventsourcingdb.SetWaitingForProbe(client, func() {
			waiting <- struct{}{}
		})

		errs := make(chan error, requests)
		go func() {
			errs <- client.Ping(context.Background())
		}()
		receive(t, server.probeReceived, "the first request did not ask")

		for range requests - 1 {
			go func() {
				errs <- client.Ping(context.Background())
			}()
		}
		for range requests - 1 {
			receive(t, waiting, "a request did not wait for the answer")
		}
		release()

		for range requests {
			assert.NoError(t, <-errs)
		}
		assert.Equal(t, int32(1), server.probes.Load())
		assert.Equal(t, "v1", eventsourcingdb.APIOf(client))
	})

	t.Run("fails every request that waits for an answer with the error of asking", func(t *testing.T) {
		released, release := newRelease()
		server := newProbedServer(t, func(w http.ResponseWriter, r *http.Request) bool {
			<-released
			return answerWithStatus(http.StatusServiceUnavailable)(w, r)
		})
		client := server.client(t)
		// The server waits for its handlers when it closes, so they have to be
		// released first, also if the test fails.
		t.Cleanup(release)

		waiting := make(chan struct{}, 1)
		eventsourcingdb.SetWaitingForProbe(client, func() {
			waiting <- struct{}{}
		})

		pingErr := make(chan error, 1)
		go func() {
			pingErr <- client.Ping(context.Background())
		}()
		receive(t, server.probeReceived, "the first request did not ask")

		writeErr := make(chan error, 1)
		go func() {
			_, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, nil)
			writeErr <- err
		}()
		receive(t, waiting, "the write did not wait for the answer")
		release()

		// Each request names its own action.
		assert.EqualError(t, <-pingErr, "failed to ping, got HTTP status code '503', expected '200': failed")
		assert.EqualError(t, <-writeErr, "failed to write events, got HTTP status code '503', expected '200': failed")
		assert.Equal(t, int32(1), server.probes.Load())
		assert.Empty(t, eventsourcingdb.APIOf(client))
	})

	t.Run("a request that waits for an answer ends with its own context", func(t *testing.T) {
		released, release := newRelease()
		server := newProbedServer(t, func(w http.ResponseWriter, r *http.Request) bool {
			select {
			case <-released:
			case <-r.Context().Done():
				return false
			}
			return answerAsV2(w, r)
		})
		client := server.client(t)
		// The server waits for its handlers when it closes, so they have to be
		// released first, also if the test fails.
		t.Cleanup(release)

		waiting := make(chan struct{}, 1)
		eventsourcingdb.SetWaitingForProbe(client, func() {
			waiting <- struct{}{}
		})

		firstErr := make(chan error, 1)
		go func() {
			firstErr <- client.Ping(context.Background())
		}()
		receive(t, server.probeReceived, "the first request did not ask")

		ctx, cancel := context.WithCancel(context.Background())
		waitingErr := make(chan error, 1)
		go func() {
			waitingErr <- client.Ping(ctx)
		}()
		receive(t, waiting, "the second request did not wait for the answer")
		cancel()

		assert.ErrorIs(t, <-waitingErr, context.Canceled)

		release()
		assert.NoError(t, <-firstErr)
		assert.Equal(t, "v2", eventsourcingdb.APIOf(client))
	})

	t.Run("a request that waits for an answer asks itself if the request that asked ends with its context", func(t *testing.T) {
		var probes atomic.Int32
		server := newProbedServer(t, func(w http.ResponseWriter, r *http.Request) bool {
			// The first request to ask never gets an answer.
			if probes.Add(1) == 1 {
				<-r.Context().Done()
				return false
			}
			return answerAsV1(w, r)
		})
		client := server.client(t)

		waiting := make(chan struct{}, 1)
		eventsourcingdb.SetWaitingForProbe(client, func() {
			waiting <- struct{}{}
		})

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		firstErr := make(chan error, 1)
		go func() {
			firstErr <- client.Ping(ctx)
		}()
		receive(t, server.probeReceived, "the first request did not ask")

		secondErr := make(chan error, 1)
		go func() {
			secondErr <- client.Ping(context.Background())
		}()
		receive(t, waiting, "the second request did not wait for the answer")
		cancel()

		assert.ErrorIs(t, <-firstErr, context.Canceled)
		assert.NoError(t, <-secondErr)
		assert.Equal(t, int32(2), server.probes.Load())
		assert.Equal(t, "v1", eventsourcingdb.APIOf(client))
	})

	t.Run("does not keep any request waiting if asking panics", func(t *testing.T) {
		// A client without a base URL panics when it sends a request.
		client, err := eventsourcingdb.NewClient(nil, "secret")
		require.NoError(t, err)

		for range 2 {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			panicked := make(chan any, 1)
			go func() {
				defer func() {
					panicked <- recover()
				}()

				_ = client.Ping(ctx)
			}()

			select {
			case value := <-panicked:
				assert.NotNil(t, value, "the request did not panic")
			case <-time.After(10 * time.Second):
				require.FailNow(t, "the request did not end")
			}
			assert.NoError(t, ctx.Err(), "the request waited until its context ended")
		}
	})

	t.Run("asks nothing for a request whose context has already ended", func(t *testing.T) {
		server := newProbedServer(t, answerAsV2)
		client := server.client(t)

		for name, request := range requestsWithContextOf(client) {
			t.Run(name, func(t *testing.T) {
				ended, cancel := context.WithCancel(context.Background())
				cancel()

				err := request(ended)

				assert.ErrorIs(t, err, context.Canceled)
				assert.Empty(t, server.requests())
			})
		}
	})
}

// probeAnswer answers the request with which a client asks a server for its
// API, and returns whether it told the client that the server offers the v2
// API.
type probeAnswer func(w http.ResponseWriter, r *http.Request) bool

// answerAsV2 answers as a server with the v2 API, with the event of a ping.
func answerAsV2(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Server", "EventSourcingDB/test")
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(apiV2.pingAnswer()))

	return true
}

// answerAsV1 answers as a server with the v1 API only, with status 404.
func answerAsV1(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Server", "EventSourcingDB/test")
	http.NotFound(w, r)

	return false
}

// answerWithStatus answers as EventSourcingDB, with the given status, and the
// description "failed".
func answerWithStatus(status int) probeAnswer {
	return func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Server", "EventSourcingDB/test")
		w.WriteHeader(status)
		_, _ = w.Write([]byte("failed"))

		return false
	}
}

// answerFromProxy answers as a proxy in front of the database that does not
// pass on the request, with the given status, and with a Server header of its
// own.
func answerFromProxy(status int) probeAnswer {
	return func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Server", "nginx/1.29.0")
		w.WriteHeader(status)
		_, _ = w.Write([]byte("<html></html>"))

		return false
	}
}

// assertUnexpectedProbeAnswer asserts that err reports an answer with status
// 200 to the request with which a client asks a server for its API that is
// not the event of a ping.
func assertUnexpectedProbeAnswer(t *testing.T, err error) {
	t.Helper()

	assert.EqualError(t, err, "failed to read events, as the server answered /api/v2/system/ping with status 200, but without the event of a ping, so it is unclear whether it offers the v2 API")
}

// newRelease returns a channel, and a function that closes it, and that can
// be called more than once.
func newRelease() (<-chan struct{}, func()) {
	released := make(chan struct{})
	var once sync.Once

	return released, func() {
		once.Do(func() {
			close(released)
		})
	}
}

// receive waits for a value of the given channel, and fails the test if none
// arrives within a few seconds.
func receive[T any](t *testing.T, values <-chan T, message string) {
	t.Helper()

	select {
	case <-values:
	case <-time.After(5 * time.Second):
		require.FailNow(t, message)
	}
}

// probedServer is a fake server that answers the request with which a client
// asks it for its API as the test says, and every other request as
// answerAsServer does. Once it has told a client that it offers the v2 API, it
// takes every ping for a ping rather than for that request. It records every
// request it receives.
type probedServer struct {
	baseURL       *url.URL
	probes        atomic.Int32
	probeReceived chan struct{}
	offersV2      atomic.Bool

	mu       sync.Mutex
	answer   probeAnswer
	received []receivedRequest
}

type receivedRequest struct {
	method        string
	path          string
	authorization string
}

func (r receivedRequest) String() string {
	return r.method + " " + r.path
}

// newProbedServer starts a probedServer, which answers the request with which
// a client asks for its API with answer.
func newProbedServer(t *testing.T, answer probeAnswer) *probedServer {
	t.Helper()

	server := &probedServer{
		answer:        answer,
		probeReceived: make(chan struct{}, 100),
	}

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.mu.Lock()
		server.received = append(server.received, receivedRequest{
			method:        r.Method,
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
		})
		answer := server.answer
		server.mu.Unlock()

		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/system/ping" || server.offersV2.Load() {
			answerAsServer(w, r)
			return
		}

		server.probes.Add(1)
		server.probeReceived <- struct{}{}
		if answer(w, r) {
			server.offersV2.Store(true)
		}
	}))
	t.Cleanup(httpServer.Close)

	baseURL, err := url.Parse(httpServer.URL)
	require.NoError(t, err)
	server.baseURL = baseURL

	return server
}

// client returns a client of the server, configured by the given options.
func (s *probedServer) client(t *testing.T, options ...eventsourcingdb.ClientOption) *eventsourcingdb.Client {
	t.Helper()

	client, err := eventsourcingdb.NewClient(s.baseURL, "secret", options...)
	require.NoError(t, err)

	return client
}

// answerProbe makes the server answer the request with which a client asks for
// its API with answer from now on.
func (s *probedServer) answerProbe(answer probeAnswer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.answer = answer
}

// requests returns every request the server received so far.
func (s *probedServer) requests() []receivedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]receivedRequest{}, s.received...)
}

// paths returns the method and the path of every request the server received
// so far.
func (s *probedServer) paths() []string {
	paths := []string{}
	for _, request := range s.requests() {
		paths = append(paths, request.String())
	}

	return paths
}
