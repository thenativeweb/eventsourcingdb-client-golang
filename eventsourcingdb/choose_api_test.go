package eventsourcingdb_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
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

	t.Run("leaves a client comparable, and a copy of it speaks the API it found out", func(t *testing.T) {
		assert.True(t, reflect.TypeFor[eventsourcingdb.Client]().Comparable(), "a client is not comparable")

		server := newProbedServer(t, answerAsV2)
		client := server.client(t)

		err := client.Ping(context.Background())
		require.NoError(t, err)

		copied := *client
		err = copied.Ping(context.Background())
		require.NoError(t, err)

		assert.Equal(t, "v2", eventsourcingdb.APIOf(&copied))
		assert.Equal(t, int32(1), server.probes.Load())
	})

	t.Run("names the path with which it asks if the server can not be reached", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		require.NoError(t, listener.Close())

		baseURL, err := url.Parse("http://" + address)
		require.NoError(t, err)
		client, err := eventsourcingdb.NewClient(baseURL, "secret")
		require.NoError(t, err)

		_, err = client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, nil)

		assert.ErrorContains(t, err, `Get "http://`+address+`/api/v2/system/ping": `)
		assert.NotErrorIs(t, err, eventsourcingdb.ErrOutcomeUnknown)
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

	t.Run("forgets the v2 API once the server answers a path of it with 404 or 405, and asks again on the next request", func(t *testing.T) {
		for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
			for _, name := range namesOfRequests() {
				t.Run(fmt.Sprintf("%d/%s", status, name), func(t *testing.T) {
					server := newChangingServer(t, answerAsServer)
					client := server.client(t)

					err := client.Ping(context.Background())
					require.NoError(t, err)
					require.Equal(t, "v2", eventsourcingdb.APIOf(client))

					server.answerWith(answerAsRolledBack(status))
					before := len(server.paths())
					request := requestOf(t, client, name)

					// The request fails as with any other status, and a write
					// may have been stored.
					err = request.send()
					assertErrorOfRequest(t, name, err, fmt.Sprintf("failed to %s, got HTTP status code '%d', expected '200'", request.action, status))
					assert.Empty(t, eventsourcingdb.APIOf(client))

					err = request.send()
					if name == "ObserveEvents" {
						assert.EqualError(t, err, "failed to observe events, the stream broke off")
					} else {
						assert.NoError(t, err)
					}
					assert.Equal(t, "v1", eventsourcingdb.APIOf(client))

					paths := server.paths()[before:]
					require.Len(t, paths, 3)
					assert.Contains(t, paths[0], " /api/v2/")
					assert.Equal(t, "GET /api/v2/system/ping", paths[1])
					assert.Contains(t, paths[2], " /api/v1/")
				})
			}
		}
	})

	t.Run("forgets the v2 API also on a 404 or 405 that does not come from EventSourcingDB, as when it asks", func(t *testing.T) {
		for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
			for _, name := range namesOfRequests() {
				t.Run(fmt.Sprintf("%d/%s", status, name), func(t *testing.T) {
					server := newChangingServer(t, answerAsServer)
					client := server.client(t)

					err := client.Ping(context.Background())
					require.NoError(t, err)
					require.Equal(t, "v2", eventsourcingdb.APIOf(client))

					// The server is rolled back behind a proxy that passes on
					// nothing but the paths of the v1 API, and answers every
					// other path itself.
					server.answerWith(func(w http.ResponseWriter, r *http.Request) {
						if strings.HasPrefix(r.URL.Path, "/api/v2/") {
							answerFromProxy(status)(w, r)
							return
						}
						answerAsServer(w, r)
					})
					before := len(server.paths())
					request := requestOf(t, client, name)

					// The request fails as before, since the answer does not
					// come from EventSourcingDB.
					err = request.send()
					assert.ErrorIs(t, err, eventsourcingdb.ErrInvalidServerHeader)
					assertErrorOfRequest(t, name, err, "server must be EventSourcingDB")
					assert.Empty(t, eventsourcingdb.APIOf(client))

					err = request.send()
					if name == "ObserveEvents" {
						assert.EqualError(t, err, "failed to observe events, the stream broke off")
					} else {
						assert.NoError(t, err)
					}
					assert.Equal(t, "v1", eventsourcingdb.APIOf(client))

					paths := server.paths()[before:]
					require.Len(t, paths, 3)
					assert.Contains(t, paths[0], " /api/v2/")
					assert.Equal(t, "GET /api/v2/system/ping", paths[1])
					assert.Contains(t, paths[2], " /api/v1/")
				})
			}
		}
	})

	t.Run("forgets nothing on an error that the v2 API reports with status 200 or with a status other than 404 and 405, nor on a 404 of the v1 API", func(t *testing.T) {
		server := newChangingServer(t, answerAsServer)
		client := server.client(t)

		err := client.Ping(context.Background())
		require.NoError(t, err)

		server.answerWith(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "EventSourcingDB/test")
			_, _ = w.Write([]byte(errorItem(eventsourcingdb.CodeServerShuttingDown) + "\n"))
		})
		for _, request := range requestsOf(client) {
			err := request.send()
			assertDBAPIError(t, err, http.StatusOK, eventsourcingdb.CodeServerShuttingDown, "failed")
			assert.Equal(t, "v2", eventsourcingdb.APIOf(client), request.name)
		}

		for _, status := range []int{http.StatusConflict, http.StatusInternalServerError} {
			server.answerWith(func(w http.ResponseWriter, r *http.Request) {
				answerWithStatus(status)(w, r)
			})
			for _, request := range requestsOf(client) {
				err := request.send()
				assertDBAPIError(t, err, status, "", "failed")
				assert.Equal(t, "v2", eventsourcingdb.APIOf(client), request.name)
			}
		}

		// Once the server is rolled back, its 404 makes the client find out
		// that it offers the v1 API only.
		server.answerWith(answerAsRolledBack(http.StatusNotFound))
		_ = client.Ping(context.Background())
		err = client.Ping(context.Background())
		require.NoError(t, err)
		require.Equal(t, "v1", eventsourcingdb.APIOf(client))

		// A 404 of the v1 API is an error of the request, e.g. an event type
		// that does not exist.
		server.answerWith(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "EventSourcingDB/test")
			http.NotFound(w, r)
		})
		before := len(server.paths())
		for _, request := range requestsOf(client) {
			err := request.send()
			var dbAPIError *eventsourcingdb.DBAPIError
			if assert.ErrorAs(t, err, &dbAPIError, request.name) {
				assert.Equal(t, http.StatusNotFound, dbAPIError.Status, request.name)
			}
			assert.Equal(t, "v1", eventsourcingdb.APIOf(client), request.name)
		}

		paths := server.paths()[before:]
		assert.Len(t, paths, len(requestsOf(nil)))
		for _, path := range paths {
			assert.Contains(t, path, " /api/v1/")
		}
	})

	t.Run("a 404 that arrives while the client asks again leaves the asking alone", func(t *testing.T) {
		released, release := newRelease()
		probeReleased, releaseProbe := newRelease()

		var isRolledBack atomic.Bool
		var probes atomic.Int32
		probeReceived := make(chan struct{}, 10)
		reading := make(chan struct{}, 10)
		statuses := make(chan int)

		client := newClientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			// Reading events through the v2 API waits for the status that the
			// test answers it with.
			case r.URL.Path == "/api/v2/data/read-events":
				reading <- struct{}{}
				select {
				case status := <-statuses:
					w.Header().Set("Server", "EventSourcingDB/test")
					w.WriteHeader(status)
				case <-released:
				case <-r.Context().Done():
				}
			// Once rolled back, the server answers the request with which the
			// client asks for its API only when the test releases it.
			case r.URL.Path == "/api/v2/system/ping" && isRolledBack.Load():
				probes.Add(1)
				probeReceived <- struct{}{}
				select {
				case <-probeReleased:
				case <-released:
				case <-r.Context().Done():
				}
				answerAsV1(w, r)
			default:
				answerAsServer(w, r)
			}
		}))
		// The server waits for its handlers when it closes, so they have to be
		// released first, also if the test fails.
		t.Cleanup(release)

		readEvents := func() error {
			_, err := countItems(client.ReadEvents(context.Background(), "/test", eventsourcingdb.ReadEventsOptions{}), nil)
			return err
		}

		err := client.Ping(context.Background())
		require.NoError(t, err)

		readErrs := make(chan error, 2)
		for range 2 {
			go func() {
				readErrs <- readEvents()
			}()
		}
		for range 2 {
			receive(t, reading, "a request did not read events through the v2 API")
		}

		// The server is rolled back, and answers the first read with 404.
		isRolledBack.Store(true)
		statuses <- http.StatusNotFound
		assertDBAPIError(t, <-readErrs, http.StatusNotFound, "", "")
		require.Empty(t, eventsourcingdb.APIOf(client))

		waiting := make(chan struct{}, 10)
		eventsourcingdb.SetWaitingForProbe(client, func() {
			waiting <- struct{}{}
		})

		askingErrs := make(chan error, 3)
		go func() {
			askingErrs <- readEvents()
		}()
		receive(t, probeReceived, "the client did not ask again")
		go func() {
			askingErrs <- readEvents()
		}()
		receive(t, waiting, "a request did not wait for the answer")

		// The 404 of the second read arrives while the client asks, and a
		// request that starts after it still waits for the answer.
		statuses <- http.StatusNotFound
		assertDBAPIError(t, <-readErrs, http.StatusNotFound, "", "")
		go func() {
			askingErrs <- readEvents()
		}()
		receive(t, waiting, "a request did not wait for the answer")

		releaseProbe()
		for range 3 {
			assert.NoError(t, <-askingErrs)
		}
		assert.Equal(t, int32(1), probes.Load())
		assert.Equal(t, "v1", eventsourcingdb.APIOf(client))
	})

	t.Run("a 404 that arrives after the client asked again leaves the newer answer alone", func(t *testing.T) {
		released, release := newRelease()

		var pings atomic.Int32
		reading := make(chan struct{}, 10)
		statuses := make(chan int)

		client := newClientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Reading events through the v2 API waits for the status that the
			// test answers it with.
			if r.URL.Path == "/api/v2/data/read-events" {
				reading <- struct{}{}
				select {
				case status := <-statuses:
					w.Header().Set("Server", "EventSourcingDB/test")
					w.WriteHeader(status)
				case <-released:
				case <-r.Context().Done():
				}
				return
			}

			if r.URL.Path == "/api/v2/system/ping" {
				pings.Add(1)
			}
			answerAsServer(w, r)
		}))
		// The server waits for its handlers when it closes, so they have to be
		// released first, also if the test fails.
		t.Cleanup(release)

		readErrs := make(chan error, 2)
		readEvents := func() {
			_, err := countItems(client.ReadEvents(context.Background(), "/test", eventsourcingdb.ReadEventsOptions{}), nil)
			readErrs <- err
		}

		err := client.Ping(context.Background())
		require.NoError(t, err)

		for range 2 {
			go readEvents()
		}
		for range 2 {
			receive(t, reading, "a request did not read events through the v2 API")
		}

		statuses <- http.StatusNotFound
		assertDBAPIError(t, <-readErrs, http.StatusNotFound, "", "")
		require.Empty(t, eventsourcingdb.APIOf(client))

		// The client asks again, and the server offers the v2 API, e.g. as it
		// was upgraded again.
		err = client.Ping(context.Background())
		require.NoError(t, err)
		require.Equal(t, "v2", eventsourcingdb.APIOf(client))
		require.Equal(t, int32(4), pings.Load())

		// The 404 of the second read was sent before.
		statuses <- http.StatusNotFound
		assertDBAPIError(t, <-readErrs, http.StatusNotFound, "", "")
		assert.Equal(t, "v2", eventsourcingdb.APIOf(client))

		err = client.Ping(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int32(5), pings.Load())
	})

	// Only the v2 API can check a recursive precondition, and a server that
	// did not offer it when the client asked may have been upgraded since,
	// e.g. while the application runs.
	t.Run("asks again before it refuses a write with a recursive precondition, and writes it through the v2 API if the server offers it now", func(t *testing.T) {
		server := newChangingServer(t, answerAsRolledBack(http.StatusNotFound))
		client := server.client(t)

		err := client.Ping(context.Background())
		require.NoError(t, err)
		require.Equal(t, "v1", eventsourcingdb.APIOf(client))

		// The server is upgraded to a version with the v2 API.
		bodies := make(chan string, 10)
		server.answerWith(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v2/data/write-events" {
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				bodies <- string(body)
			}

			answerAsServer(w, r)
		})
		before := len(server.paths())

		writtenEvents, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, []eventsourcingdb.Precondition{
			eventsourcingdb.NewIsSubjectPristinePrecondition("/test", eventsourcingdb.PreconditionOptions{Recursive: true}),
		})

		require.NoError(t, err)
		assert.Len(t, writtenEvents, 1)
		assert.Equal(t, []string{"GET /api/v2/system/ping", "POST /api/v2/data/write-events"}, server.paths()[before:])
		assert.JSONEq(t, `{
			"events": [{"source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","data":{"value":23}}],
			"preconditions": [{"type":"isSubjectPristine","payload":{"subject":"/test","options":{"recursive":true}}}]
		}`, <-bodies)
		assert.Equal(t, "v2", eventsourcingdb.APIOf(client))

		// The requests after it speak the v2 API, without asking again.
		before = len(server.paths())
		_, err = client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"POST /api/v2/data/write-events"}, server.paths()[before:])
	})

	t.Run("asks again before it refuses a write with a recursive precondition, and refuses it without sending it if the server still does not offer the v2 API", func(t *testing.T) {
		server := newChangingServer(t, answerAsRolledBack(http.StatusNotFound))
		client := server.client(t)

		err := client.Ping(context.Background())
		require.NoError(t, err)
		require.Equal(t, "v1", eventsourcingdb.APIOf(client))
		before := len(server.paths())

		_, err = client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, []eventsourcingdb.Precondition{
			eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test", eventsourcingdb.PreconditionOptions{Recursive: true}),
		})

		assert.ErrorIs(t, err, eventsourcingdb.ErrIncompatibleServer)
		assert.EqualError(t, err, "the server does not offer the v2 API, which a recursive precondition needs")
		assert.Equal(t, []string{"GET /api/v2/system/ping"}, server.paths()[before:])
		assert.Equal(t, "v1", eventsourcingdb.APIOf(client))

		// A write without a recursive precondition goes on with the answer,
		// without asking again.
		before = len(server.paths())
		_, err = client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, []eventsourcingdb.Precondition{
			eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test", eventsourcingdb.PreconditionOptions{Recursive: false}),
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"POST /api/v1/write-events"}, server.paths()[before:])
	})

	t.Run("asks again once for writes with a recursive precondition that start while it asks, and sends each of them once", func(t *testing.T) {
		const writes = 5

		released, release := newRelease()
		var isUpgraded atomic.Bool
		var probes atomic.Int32
		probeReceived := make(chan struct{}, 10)

		server := newChangingServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v2/system/ping" {
				probes.Add(1)
				if !isUpgraded.Load() {
					answerAsV1(w, r)
					return
				}

				// Once upgraded, the server answers the request with which the
				// client asks for its API only when the test releases it.
				probeReceived <- struct{}{}
				select {
				case <-released:
				case <-r.Context().Done():
					return
				}
				answerAsV2(w, r)
				return
			}

			if !isUpgraded.Load() && strings.HasPrefix(r.URL.Path, "/api/v2/") {
				answerAsV1(w, r)
				return
			}

			answerAsServer(w, r)
		})
		client := server.client(t)
		// The server waits for its handlers when it closes, so they have to be
		// released first, also if the test fails.
		t.Cleanup(release)

		err := client.Ping(context.Background())
		require.NoError(t, err)
		require.Equal(t, "v1", eventsourcingdb.APIOf(client))

		isUpgraded.Store(true)

		waiting := make(chan struct{}, writes)
		eventsourcingdb.SetWaitingForProbe(client, func() {
			waiting <- struct{}{}
		})

		writeRecursively := func() error {
			_, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, []eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "0", eventsourcingdb.PreconditionOptions{Recursive: true}),
			})
			return err
		}

		errs := make(chan error, writes)
		go func() {
			errs <- writeRecursively()
		}()
		receive(t, probeReceived, "the client did not ask again")

		for range writes - 1 {
			go func() {
				errs <- writeRecursively()
			}()
		}
		for range writes - 1 {
			receive(t, waiting, "a write did not wait for the answer")
		}
		release()

		for range writes {
			assert.NoError(t, <-errs)
		}
		assert.Equal(t, int32(2), probes.Load())
		assert.Equal(t, "v2", eventsourcingdb.APIOf(client))

		sentWrites := []string{}
		for _, path := range server.paths() {
			if strings.HasSuffix(path, "/write-events") {
				sentWrites = append(sentWrites, path)
			}
		}
		assert.Equal(t, slices.Repeat([]string{"POST /api/v2/data/write-events"}, writes), sentWrites)
	})
}

// answerAsRolledBack answers as a server that was rolled back to a version
// without the v2 API: every path of the v2 API with the given status, and
// every other one as answerAsServer does.
func answerAsRolledBack(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v2/") {
			answerAsServer(w, r)
			return
		}

		w.Header().Set("Server", "EventSourcingDB/test")
		w.WriteHeader(status)
	}
}

// changingServer is a fake server that answers every request with the handler
// that the test gave it last, e.g. to roll it back to a version without the v2
// API. It records the method and the path of every request it receives.
type changingServer struct {
	baseURL *url.URL

	mu       sync.Mutex
	answer   http.HandlerFunc
	received []string
}

// newChangingServer starts a changingServer, which answers every request with
// answer.
func newChangingServer(t *testing.T, answer http.HandlerFunc) *changingServer {
	t.Helper()

	server := &changingServer{answer: answer}

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.mu.Lock()
		server.received = append(server.received, r.Method+" "+r.URL.Path)
		answer := server.answer
		server.mu.Unlock()

		answer(w, r)
	}))
	t.Cleanup(httpServer.Close)

	baseURL, err := url.Parse(httpServer.URL)
	require.NoError(t, err)
	server.baseURL = baseURL

	return server
}

// client returns a client of the server.
func (s *changingServer) client(t *testing.T) *eventsourcingdb.Client {
	t.Helper()

	client, err := eventsourcingdb.NewClient(s.baseURL, "secret")
	require.NoError(t, err)

	return client
}

// answerWith makes the server answer every request with answer from now on.
func (s *changingServer) answerWith(answer http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.answer = answer
}

// paths returns the method and the path of every request the server received
// so far.
func (s *changingServer) paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string{}, s.received...)
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
