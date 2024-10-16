package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/httpserver"
)

func TestObserveEvents(t *testing.T) {
	janeRegistered := eventsourcingdb.NewEventCandidate(
		events.TestSource,
		"/users/registered",
		events.Events.Registered.JaneDoe.Type,
		events.Events.Registered.JaneDoe.Data,
	).WithTraceParent(events.Events.Registered.JaneDoe.TraceParent)

	johnRegistered := eventsourcingdb.NewEventCandidate(
		events.TestSource,
		"/users/registered",
		events.Events.Registered.JohnDoe.Type,
		events.Events.Registered.JohnDoe.Data,
	).WithTraceParent(events.Events.Registered.JohnDoe.TraceParent)

	janeLoggedIn := eventsourcingdb.NewEventCandidate(
		events.TestSource,
		"/users/loggedIn",
		events.Events.LoggedIn.JaneDoe.Type,
		events.Events.LoggedIn.JaneDoe.Data,
	).WithTraceParent(events.Events.LoggedIn.JaneDoe.TraceParent)

	johnLoggedIn := eventsourcingdb.NewEventCandidate(
		events.TestSource,
		"/users/loggedIn",
		events.Events.LoggedIn.JohnDoe.Type,
		events.Events.LoggedIn.JohnDoe.Data,
	).WithTraceParent(events.Events.LoggedIn.JohnDoe.TraceParent)

	prepareClientWithEvents := func(t *testing.T) eventsourcingdb.Client {
		client := database.WithAuthorization.GetClient()

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			janeRegistered,
			janeLoggedIn,
			johnRegistered,
			johnLoggedIn,
		})

		assert.NoError(t, err)

		return client
	}

	assertRegisteredEvent := func(t *testing.T, event eventsourcingdb.Event, expected events.RegisteredEvent) {
		assert.Equal(t, "/users/registered", event.Subject)
		assert.Equal(t, expected.Type, event.Type)
		assert.Equal(t, expected.TraceParent, *event.TraceParent)

		var eventData events.RegisteredEventData
		err := json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	assertLoggedInEvent := func(t *testing.T, event eventsourcingdb.Event, expected events.LoggedInEvent) {
		assert.Equal(t, "/users/loggedIn", event.Subject)
		assert.Equal(t, expected.Type, event.Type)
		assert.Equal(t, expected.TraceParent, *event.TraceParent)

		var eventData events.LoggedInEventData
		err := json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	t.Run("returns a server error when trying to observe from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, err := test.Take(1, client.ObserveEvents(ctx, "/", eventsourcingdb.ObserveNonRecursively()))

		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
	})

	t.Run("observes events from a single subject.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(100 * time.Millisecond)

			apfelFredCandidate := eventsourcingdb.NewEventCandidate(
				events.TestSource,
				"/users/registered",
				events.Events.Registered.ApfelFred.Type,
				events.Events.Registered.ApfelFred.Data,
			).WithTraceParent(events.Events.Registered.ApfelFred.TraceParent)

			_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
				apfelFredCandidate,
			})
			assert.NoError(t, err)

		}()

		count := 0
	loop:
		for result, err := range client.ObserveEvents(ctx, "/users/registered", eventsourcingdb.ObserveNonRecursively()) {
			assert.NoError(t, err)
			count++

			switch count {
			case 1:
				assertRegisteredEvent(t, result.Event, events.Events.Registered.JaneDoe)
			case 2:
				assertRegisteredEvent(t, result.Event, events.Events.Registered.JohnDoe)
			case 3:
				assertRegisteredEvent(t, result.Event, events.Events.Registered.ApfelFred)
				break loop
			}
		}
	})

	t.Run("observes events from a subject including child subjects.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		results, err := test.Take(4, client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
		))
		assert.NoError(t, err)

		assertRegisteredEvent(t, results[0].Event, events.Events.Registered.JaneDoe)
		assertLoggedInEvent(t, results[1].Event, events.Events.LoggedIn.JaneDoe)
		assertRegisteredEvent(t, results[2].Event, events.Events.Registered.JohnDoe)
		assertLoggedInEvent(t, results[3].Event, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("observes events starting from the newest event matching the given event name.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		results, err := test.Take(1, client.ObserveEvents(
			ctx,
			"/users/loggedIn",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLatestEvent(
				"/users/loggedIn",
				events.PrefixEventType("loggedIn"),
				eventsourcingdb.IfEventIsMissingDuringObserveReadEverything,
			),
		))
		assert.NoError(t, err)

		assertLoggedInEvent(t, results[0].Event, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("observes events starting from the lower bound ID.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		results, err := test.Take(2, client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("2"),
		))
		assert.NoError(t, err)

		assertRegisteredEvent(t, results[0].Event, events.Events.Registered.JohnDoe)
		assertLoggedInEvent(t, results[1].Event, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("returns a ContextCanceledError when the context is canceled before the request is sent.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())

		cancel()

		_, err := test.Take(1, client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("2"),
		))

		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns a ContextCanceledError when the context is canceled while reading the ndjson stream.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var err error
		count := 0
		for _, err = range client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("3"),
		) {
			count++
			if count == 1 {
				assert.NoError(t, err)
				cancel()
			}
		}

		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns a ContextCanceledError when the context is canceled while waiting for new events.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var err error
		count := 0
		for _, err = range client.ObserveEvents(
			ctx,
			"/",
			eventsourcingdb.ObserveRecursively(),
		) {
			count++
			if count == 4 {
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
			}
		}

		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns an error if mutually exclusive options are used", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := test.Take(1, client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("0"),
			eventsourcingdb.ObserveFromLatestEvent("/", "com.foo.bar", eventsourcingdb.IfEventIsMissingDuringObserveWaitForEvent),
		))

		assert.ErrorContains(t, err, "argument 'ObserveFromLatestEvent' is invalid: ObserveFromLowerBoundID and ObserveFromLatestEvent are mutually exclusive")
	})

	t.Run("returns an error if the given lowerBoundID does not contain an integer.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := test.Take(1, client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("lmao"),
		))

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "argument 'ObserveFromLowerBoundID' is invalid: lowerBoundID must contain an integer")
	})

	t.Run("returns an error if the given lowerBoundID contains an integer that is negative.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := test.Take(1, client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("-1"),
		))

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "argument 'ObserveFromLowerBoundID' is invalid: lowerBoundID must be 0 or greater")
	})

	t.Run("returns an error if an incorrect subject is used in ObserveFromLatestEvent.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := test.Take(1, client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLatestEvent("", "com.foo.bar", eventsourcingdb.IfEventIsMissingDuringObserveWaitForEvent),
		))

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "argument 'ObserveFromLatestEvent' is invalid: malformed event subject")
	})

	t.Run("returns an error if an incorrect type is used in ObserveFromLatestEvent.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := test.Take(1, client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLatestEvent("/", ".bar", eventsourcingdb.IfEventIsMissingDuringObserveWaitForEvent),
		))

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "argument 'ObserveFromLatestEvent' is invalid: malformed event type")
	})

	t.Run("returns a sever error if the server responds with HTTP 5xx", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "Bad Gateway")
	})

	t.Run("returns an error if the server's protocol version does not match.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("X-EventSourcingDB-Protocol-Version", "0.0.0")
				writer.WriteHeader(http.StatusUnprocessableEntity)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrClientError))
		assert.ErrorContains(t, err, "protocol version mismatch, server '0.0.0', client '1.0.0'")
	})

	t.Run("returns a client error if the server returns a 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadRequest)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrClientError))
		assert.ErrorContains(t, err, "Bad Request")
	})

	t.Run("returns a server error if the server returns a non 200, 5xx or 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "unexpected response status: 202 Accepted")
	})

	t.Run("returns a server error if the server sends a stream item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": 42}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "unsupported stream item encountered: cannot unmarshal")
	})

	t.Run("returns a server error if the server sends a stream item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"clowns\": 8}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "unsupported stream item encountered:")
		assert.ErrorContains(t, err, "does not have a recognized type")
	})

	t.Run("returns a server error if the server sends an error item through the ndjson stream.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"error\", \"payload\": {\"error\": \"aliens have abducted the server\"}}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "aliens have abducted the server")
	})

	t.Run("returns a server error if the server sends an error item through the ndjson stream, but the error can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"error\", \"payload\": {\"error\": 8}}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "unexpected stream error encountered:")
	})

	t.Run("returns a server error if the server sends an item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"item\", \"payload\": {\"event\": 8}}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		_, err = test.Take(1, client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()))

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "stream item encountered:")
		assert.ErrorContains(t, err, "(trying to unmarshal")
	})

	t.Run("returns an error if the subject is invalid.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := test.Take(1, client.ObserveEvents(context.Background(), "uargh", eventsourcingdb.ObserveRecursively()))

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "argument 'subject' is invalid: malformed event subject 'uargh': subject must be an absolute, slash-separated path")
	})

	t.Run("observes for an expanded period of time.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			time.Sleep(3 * time.Second)
			apfelFredCandidate := eventsourcingdb.NewEventCandidate(events.TestSource, "/users/registered", events.Events.Registered.ApfelFred.Type, events.Events.Registered.ApfelFred.Data)
			_, _ = client.WriteEvents([]eventsourcingdb.EventCandidate{
				apfelFredCandidate,
			})
		}()

		for result, err := range client.ObserveEvents(ctx, "/", eventsourcingdb.ObserveRecursively()) {
			assert.NoError(t, err)
			assert.Equal(t, events.Events.Registered.ApfelFred.Type, result.Event.Type)
			return
		}
	})

	// Regression test for https://github.com/thenativeweb/eventsourcingdb-client-golang/pull/97
	t.Run("works with contexts that have a deadline.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Millisecond))
		defer cancel()

		time.Sleep(2 * time.Millisecond)

		_, err := test.Take(1, client.ObserveEvents(ctx, "/", eventsourcingdb.ObserveRecursively()))

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.NotErrorIs(t, eventsourcingdb.ErrServerError, err)
		assert.NotErrorIs(t, eventsourcingdb.ErrClientError, err)
		assert.NotErrorIs(t, eventsourcingdb.ErrInternalError, err)
		assert.NotContains(t, err.Error(), "unsupported stream item")
	})

	t.Run("returns an error when the client disconnects from the server.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				count := 0
				for {
					time.Sleep(1 * time.Second)

					if count < 3 {
						_, err := writer.Write([]byte("{\"type\": \"heartbeat\"}\n"))
						if err != nil {
							panic(err)
						}
					}
					if f, ok := writer.(http.Flusher); ok {
						f.Flush()
					}
					count++
				}
			})
		})
		defer stopServer()

		var err error
		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		for _, err = range client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively()) {
			// intentionally left blank
		}

		assert.ErrorIs(t, err, eventsourcingdb.ErrServerError)
		assert.ErrorContains(t, err, "heartbeat timeout")
	})
}
