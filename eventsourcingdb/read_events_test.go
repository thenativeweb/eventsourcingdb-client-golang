package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/httpserver"
	"github.com/thenativeweb/goutils/v2/platformutils"
)

func TestReadEvents(t *testing.T) {
	client := database.WithAuthorization.GetClient()

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

	_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
		janeRegistered,
		janeLoggedIn,
		johnRegistered,
		johnLoggedIn,
	})

	assert.NoError(t, err)

	getNextEvent := func(t *testing.T, resultChan <-chan eventsourcingdb.ReadEventsResult) eventsourcingdb.Event {
		firstStoreItem := <-resultChan
		data, err := firstStoreItem.GetData()

		assert.NoError(t, err)

		return data.Event
	}

	matchRegisteredEvent := func(t *testing.T, event eventsourcingdb.Event, expected events.RegisteredEvent) {
		assert.Equal(t, "/users/registered", event.Subject)
		assert.Equal(t, expected.Type, event.Type)
		assert.Equal(t, expected.TraceParent, *event.TraceParent)

		var eventData events.RegisteredEventData
		err = json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	matchLoggedInEvent := func(t *testing.T, event eventsourcingdb.Event, expected events.LoggedInEvent) {
		assert.Equal(t, "/users/loggedIn", event.Subject)
		assert.Equal(t, expected.Type, event.Type)
		assert.Equal(t, expected.TraceParent, *event.TraceParent)

		var eventData events.LoggedInEventData
		err = json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	t.Run("returns a server error when trying to read from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()

		resultChan := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadNonRecursively())

		firstResult := <-resultChan

		_, err := firstResult.GetData()
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
	})

	t.Run("reads events from a single subject.", func(t *testing.T) {
		fmt.Printf("Client: %v\n", client)
		resultChan := client.ReadEvents(context.Background(), "/users/registered", eventsourcingdb.ReadNonRecursively())

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, events.Events.Registered.JohnDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads events from a subject including child subjects.", func(t *testing.T) {
		resultChan := client.ReadEvents(
			context.Background(),
			"/users",
			eventsourcingdb.ReadRecursively(),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JaneDoe)

		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, events.Events.Registered.JohnDoe)

		fourthEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, fourthEvent, events.Events.LoggedIn.JohnDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads the events in antichronological order.", func(t *testing.T) {
		resultChan := client.ReadEvents(
			context.Background(),
			"/users/registered",
			eventsourcingdb.ReadNonRecursively(),
			eventsourcingdb.ReadAntichronologically(),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, events.Events.Registered.JaneDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads events starting from the latest event matching the given event name.", func(t *testing.T) {
		resultChan := client.ReadEvents(
			context.Background(),
			"/users/loggedIn",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadFromLatestEvent(
				"/users/loggedIn",
				events.PrefixEventType("loggedIn"),
				eventsourcingdb.IfEventIsMissingDuringReadReadEverything,
			),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, firstEvent, events.Events.LoggedIn.JohnDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads events starting from the lower bound ID.", func(t *testing.T) {
		resultChan := client.ReadEvents(
			context.Background(),
			"/users",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadFromLowerBoundID("2"),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JohnDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads events up to the upper bound ID.", func(t *testing.T) {
		resultChan := client.ReadEvents(
			context.Background(),
			"/users",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadUntilUpperBoundID("1"),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JaneDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("returns a ContextCanceledError when the context is canceled before sending the request.", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		resultChan := client.ReadEvents(
			ctx,
			"/users",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadUntilUpperBoundID("1"),
		)

		_, err := (<-resultChan).GetData()
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns a ContextCanceledError when the context is canceled while reading the ndjson stream.", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"item\", \"payload\": {}}\n")); err != nil {
					panic(err)
				}
				cancel()
			})
		})
		defer stopServer()

		client, _ := eventsourcingdb.NewClient(serverAddress, "access-token")
		resultChan := client.ReadEvents(
			ctx,
			"/",
			eventsourcingdb.ReadRecursively(),
		)

		_, err := (<-resultChan).GetData()
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns an error if mutually exclusive options are used.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadEvents(
			context.Background(),
			"/",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadFromLowerBoundID("0"),
			eventsourcingdb.ReadFromLatestEvent("/", "com.foo.bar", eventsourcingdb.IfEventIsMissingDuringReadReadEverything),
		)

		result := <-results
		_, err := result.GetData()

		assert.ErrorContains(t, err, "parameter 'ReadFromLatestEvent' is invalid: ReadFromLowerBoundID and ReadFromLatestEvent are mutually exclusive")
	})

	t.Run("returns an error if the given lowerBoundID does not contain an integer.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadEvents(
			context.Background(),
			"/",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadFromLowerBoundID("lmao"),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'ReadFromLowerBoundID' is invalid: lowerBoundID must contain an integer")
	})

	t.Run("returns an error if the given lowerBoundID contains an integer that is negative.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadEvents(
			context.Background(),
			"/",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadFromLowerBoundID("-1"),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'ReadFromLowerBoundID' is invalid: lowerBoundID must be 0 or greater")
	})

	t.Run("returns an error if the given upperBoundID does not contain an integer.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadEvents(
			context.Background(),
			"/",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadUntilUpperBoundID("lmao"),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'ReadUntilUpperBoundID' is invalid: upperBoundID must contain an integer")
	})

	t.Run("returns an error if the given upperBoundID contains an integer that is negative.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadEvents(
			context.Background(),
			"/",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadUntilUpperBoundID("-1"),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'ReadUntilUpperBoundID' is invalid: upperBoundID must be 0 or greater")
	})

	t.Run("returns an error if an incorrect subject is used in ReadFromLatestEvent.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadEvents(
			context.Background(),
			"/",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadFromLatestEvent("", "com.foo.bar", eventsourcingdb.IfEventIsMissingDuringReadReadNothing),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'ReadFromLatestEvent' is invalid: malformed event subject")
	})

	t.Run("returns an error if an incorrect type is used in ReadFromLatestEvent.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadEvents(
			context.Background(),
			"/",
			eventsourcingdb.ReadRecursively(),
			eventsourcingdb.ReadFromLatestEvent("/", ".bar", eventsourcingdb.IfEventIsMissingDuringReadReadNothing),
		)

		result := <-results
		_, err := result.GetData()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'ReadFromLatestEvent' is invalid: malformed event type")
	})

	t.Run("returns a sever error if the server responds with HTTP 5xx on every try", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "Bad Gateway")
	})

	t.Run("returns an error if the server's protocol version does not match.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("X-EventSourcingDB-Protocol-Version", "0.0.0")
				writer.WriteHeader(http.StatusUnprocessableEntity)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrClientError))
		assert.ErrorContains(t, err, "client error: protocol version mismatch, server '0.0.0', client '1.0.0'")
	})

	t.Run("returns a client error if the server returns a 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadRequest)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrClientError))
		assert.ErrorContains(t, err, "Bad Request")
	})

	t.Run("returns a server error if the server returns a non 200, 5xx or 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "unexpected response status: 202 Accepted")
	})

	t.Run("returns a server error if the server sends a stream item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": 42}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered: cannot unmarshal")
	})

	t.Run("returns a server error if the server sends a stream item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"clowns\": 8}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered:")
		assert.ErrorContains(t, err, "does not have a recognized type")
	})

	t.Run("returns a server error if the server sends a an error item through the ndjson stream.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"error\", \"payload\": {\"error\": \"aliens have abducted the server\"}}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "server error: aliens have abducted the server")
	})

	t.Run("returns a server error if the server sends a an error item through the ndjson stream, but the error can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"error\", \"payload\": {\"error\": 8}}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "server error: unsupported stream error encountered:")
	})

	t.Run("returns a server error if the server sends an item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-events", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"item\", \"payload\": {\"event\": 8}}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadEvents(context.Background(), "/", eventsourcingdb.ReadRecursively())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered:")
		assert.ErrorContains(t, err, "(trying to unmarshal")
	})

	t.Run("returns an error if the subject is invalid.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadEvents(context.Background(), "uargh", eventsourcingdb.ReadRecursively())
		_, err := (<-results).GetData()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'subject' is invalid: malformed event subject 'uargh': subject must be an absolute, slash-separated path")
	})

	// Regression test for https://github.com/thenativeweb/eventsourcingdb-client-golang/pull/97
	t.Run("Works with contexts that have a deadline.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*platformutils.Jiffy))
		defer cancel()

		time.Sleep(2 * platformutils.Jiffy)

		results := client.ReadEvents(ctx, "/", eventsourcingdb.ReadNonRecursively())
		result := <-results
		_, err := result.GetData()

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.NotErrorIs(t, eventsourcingdb.ErrServerError, err)
		assert.NotErrorIs(t, eventsourcingdb.ErrClientError, err)
		assert.NotErrorIs(t, eventsourcingdb.ErrInternalError, err)
		assert.NotContains(t, err.Error(), "unsupported stream item")
	})
}
