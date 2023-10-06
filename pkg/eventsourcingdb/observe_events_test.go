package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/thenativeweb/goutils/v2/platformutils"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/httpserver"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/ifeventismissingduringobserve"
)

func TestObserveEvents(t *testing.T) {
	janeRegistered := event.NewCandidate(
		events.TestSource,
		"/users/registered",
		events.Events.Registered.JaneDoe.Type,
		events.Events.Registered.JaneDoe.Data,
		event.WithTraceParent(events.Events.Registered.JaneDoe.TraceParent),
	)
	johnRegistered := event.NewCandidate(
		events.TestSource,
		"/users/registered",
		events.Events.Registered.JohnDoe.Type,
		events.Events.Registered.JohnDoe.Data,
		event.WithTraceParent(events.Events.Registered.JohnDoe.TraceParent),
	)
	janeLoggedIn := event.NewCandidate(
		events.TestSource,
		"/users/loggedIn",
		events.Events.LoggedIn.JaneDoe.Type,
		events.Events.LoggedIn.JaneDoe.Data,
		event.WithTraceParent(events.Events.LoggedIn.JaneDoe.TraceParent),
	)
	johnLoggedIn := event.NewCandidate(
		events.TestSource,
		"/users/loggedIn",
		events.Events.LoggedIn.JohnDoe.Type,
		events.Events.LoggedIn.JohnDoe.Data,
		event.WithTraceParent(events.Events.LoggedIn.JohnDoe.TraceParent),
	)

	prepareClientWithEvents := func(t *testing.T) eventsourcingdb.Client {
		client := database.WithAuthorization.GetClient()

		_, err := client.WriteEvents([]event.Candidate{
			janeRegistered,
			janeLoggedIn,
			johnRegistered,
			johnLoggedIn,
		})

		assert.NoError(t, err)

		return client
	}

	getNextEvent := func(t *testing.T, resultChan <-chan eventsourcingdb.ObserveEventsResult) event.Event {
		firstStoreItem := <-resultChan
		data, err := firstStoreItem.GetData()

		assert.NoError(t, err)

		return data.Event
	}

	matchRegisteredEvent := func(t *testing.T, event event.Event, expected events.RegisteredEvent) {
		assert.Equal(t, "/users/registered", event.Subject)
		assert.Equal(t, expected.Type, event.Type)
		assert.Equal(t, expected.TraceParent, *event.TraceParent)

		var eventData events.RegisteredEventData
		err := json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	matchLoggedInEvent := func(t *testing.T, event event.Event, expected events.LoggedInEvent) {
		assert.Equal(t, "/users/loggedIn", event.Subject)
		assert.Equal(t, expected.Type, event.Type)
		assert.Equal(t, expected.TraceParent, *event.TraceParent)

		var eventData events.LoggedInEventData
		err := json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	t.Run("returns an error when trying to observe from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(ctx, "/", eventsourcingdb.ObserveNonRecursively())

		firstResult := <-resultChan

		_, err := firstResult.GetData()
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "server error: retries exceeded")
	})

	t.Run("observes events from a single subject.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(ctx, "/users/registered", eventsourcingdb.ObserveNonRecursively())

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, events.Events.Registered.JohnDoe)

		apfelFredCandidate := event.NewCandidate(
			events.TestSource,
			"/users/registered",
			events.Events.Registered.ApfelFred.Type,
			events.Events.Registered.ApfelFred.Data,
			event.WithTraceParent(events.Events.Registered.ApfelFred.TraceParent),
		)
		_, err := client.WriteEvents([]event.Candidate{
			apfelFredCandidate,
		})

		assert.NoError(t, err)
		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, events.Events.Registered.ApfelFred)
	})

	t.Run("observes events from a subject including child subjects.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JaneDoe)

		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, events.Events.Registered.JohnDoe)

		fourthEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, fourthEvent, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("observes events starting from the newest event matching the given event name.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(
			ctx,
			"/users/loggedIn",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLatestEvent(
				"/users/loggedIn",
				events.PrefixEventType("loggedIn"),
				ifeventismissingduringobserve.ReadEverything,
			),
		)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("observes events starting from the lower bound ID.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("2"),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("returns a ContextCanceledError when the context is canceled before the request is sent.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())

		cancel()

		resultChan := client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("2"),
		)

		_, err := (<-resultChan).GetData()
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns a ContextCanceledError when the context is canceled while reading the ndjson stream.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())

		resultChan := client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("3"),
		)

		<-resultChan
		cancel()

		_, err := (<-resultChan).GetData()
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns an error if mutually exclusive options are used", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("0"),
			eventsourcingdb.ObserveFromLatestEvent("/", "com.foo.bar", ifeventismissingduringobserve.WaitForEvent),
		)

		result := <-results
		_, err := result.GetData()

		assert.ErrorContains(t, err, "parameter 'ObserveFromLatestEvent' is invalid\nObserveFromLowerBoundID and ObserveFromLatestEvent are mutually exclusive")
	})

	t.Run("returns an error if the given lowerBoundID does not contain an integer.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("lmao"),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'ObserveFromLowerBoundID' is invalid\nlowerBoundID must contain an integer")
	})

	t.Run("returns an error if the given lowerBoundID contains an integer that is negative.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("-1"),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'ObserveFromLowerBoundID' is invalid\nlowerBoundID must be 0 or greater")
	})

	t.Run("returns an error if an incorrect subject is used in ObserveFromLatestEvent.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLatestEvent("", "com.foo.bar", ifeventismissingduringobserve.WaitForEvent),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'ObserveFromLatestEvent' is invalid\nmalformed event subject")
	})

	t.Run("returns an error if an incorrect type is used in ObserveFromLatestEvent.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLatestEvent("/", ".bar", ifeventismissingduringobserve.WaitForEvent),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'ObserveFromLatestEvent' is invalid\nmalformed event type")
	})

	t.Run("returns a sever error if the server responds with HTTP 5xx on every try", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/observe-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token", eventsourcingdb.MaxTries(2))
		assert.NoError(t, err)

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "retries exceeded")
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

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrClientError))
		assert.ErrorContains(t, err, "client error: protocol version mismatch, server '0.0.0', client '1.0.0'")
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

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrClientError))
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

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
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

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered: cannot unmarshal")
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

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered:")
		assert.ErrorContains(t, err, "does not have a recognized type")
	})

	t.Run("returns a server error if the server sends a an error item through the ndjson stream.", func(t *testing.T) {
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

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "server error: aliens have abducted the server")
	})

	t.Run("returns a server error if the server sends a an error item through the ndjson stream, but the error can't be unmarshalled.", func(t *testing.T) {
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

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "server error: unsupported stream error encountered:")
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

		results := client.ObserveEvents(context.Background(), "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered:")
		assert.ErrorContains(t, err, "(trying to unmarshal")
	})

	t.Run("returns an error if the subject is invalid.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ObserveEvents(context.Background(), "uargh", eventsourcingdb.ObserveRecursively())
		_, err := (<-results).GetData()

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'subject' is invalid\nmalformed event subject 'uargh': subject must be an absolute, slash-separated path")
	})

	t.Run("observes for longer than ten seconds.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		results := client.ObserveEvents(ctx, "/", eventsourcingdb.ObserveRecursively())
		for {
			select {
			case _, ok := <-results:
				assert.True(t, ok)
				if !ok {
					return
				}
			case <-time.After(11 * time.Second):
				apfelFredCandidate := event.NewCandidate(events.TestSource, "/users/registered", events.Events.Registered.ApfelFred.Type, events.Events.Registered.ApfelFred.Data)
				_, _ = client.WriteEvents([]event.Candidate{
					apfelFredCandidate,
				})
				_, ok := <-results
				assert.True(t, ok)
				return
			}
		}
	})

	// Regression test for https://github.com/thenativeweb/eventsourcingdb-client-golang/pull/97
	t.Run("Works with contexts that have a deadline.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*platformutils.Jiffy))
		defer cancel()

		time.Sleep(2 * platformutils.Jiffy)

		results := client.ObserveEvents(ctx, "/", eventsourcingdb.ObserveRecursively())
		result := <-results
		_, err := result.GetData()

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.NotErrorIs(t, customErrors.ErrServerError, err)
		assert.NotErrorIs(t, customErrors.ErrClientError, err)
		assert.NotErrorIs(t, customErrors.ErrInternalError, err)
		assert.NotContains(t, err.Error(), "unsupported stream item")
	})
}
