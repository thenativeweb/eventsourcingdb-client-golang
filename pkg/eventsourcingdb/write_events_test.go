package eventsourcingdb_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/httpserver"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWriteEvents(t *testing.T) {
	t.Run("returns an error when trying to write to a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()
		source := event.NewSource(events.TestSource)

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data, event.WithTracingContext(janeRegistered.TracingContext)),
			},
		)

		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "server error\nretries exceeded")
	})

	t.Run("returns an error if no candidates are passed.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := client.WriteEvents(
			[]event.Candidate{},
		)

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'eventCandidates' is invalid\neventCandidates must contain at least one EventCandidate")
	})

	t.Run("returns an error if a candidate subject is malformed", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := client.WriteEvents(
			[]event.Candidate{
				event.NewCandidate("tag:foobar.com,2023:barbaz", "foobar", "com.foobar.barbaz", struct{}{}),
			},
		)

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'eventCandidates' is invalid\nevent candidate failed to validate: malformed event subject 'foobar': subject must be an absolute, slash-separated path")
	})

	t.Run("returns an error if a candidate type is malformed", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := client.WriteEvents(
			[]event.Candidate{
				event.NewCandidate("tag:foobar.com,2023:barbaz", "/foobar", "barbaz", struct{}{}),
			},
		)

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'eventCandidates' is invalid\nevent candidate failed to validate: malformed event type 'barbaz': type must be a reverse domain name")
	})

	t.Run("returns an error if a candidate source is malformed", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := client.WriteEvents(
			[]event.Candidate{
				event.NewCandidate("://foobar", "/foobar", "com.foobar.barbaz", struct{}{}),
			},
		)

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'eventCandidates' is invalid\nevent candidate failed to validate: malformed event source '://foobar': source must be a valid URI")
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data, event.WithTracingContext(janeRegistered.TracingContext)),
			},
		)

		assert.NoError(t, err)
	})

	t.Run("writes a single event.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data, event.WithTracingContext(janeRegistered.TracingContext)),
			},
		)

		assert.NoError(t, err)
	})

	t.Run("returns the written event metadata.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		janeRegistered := events.Events.Registered.JaneDoe
		johnRegistered := events.Events.Registered.JohnDoe
		johnLoggedIn := events.Events.LoggedIn.JohnDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent("/users/registered", janeRegistered.Type, janeRegistered.Data, event.WithTracingContext(janeRegistered.TracingContext)),
			},
		)
		assert.NoError(t, err)

		writtenEventsMetadata, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent("/users/registered", johnRegistered.Type, johnRegistered.Data, event.WithTracingContext(johnRegistered.TracingContext)),
				source.NewEvent("/users/loggedIn", johnLoggedIn.Type, johnLoggedIn.Data, event.WithTracingContext(johnLoggedIn.TracingContext)),
			},
		)

		assert.Len(t, writtenEventsMetadata, 2)
		assert.Equal(t, events.TestSource, writtenEventsMetadata[0].Source)
		assert.Equal(t, events.PrefixEventType("registered"), writtenEventsMetadata[0].Type)
		assert.Equal(t, "/users/registered", writtenEventsMetadata[0].Subject)
		assert.Equal(t, "1", writtenEventsMetadata[0].ID)
		assert.Equal(t, johnRegistered.TracingContext, writtenEventsMetadata[0].TracingContext)
		assert.Equal(t, events.TestSource, writtenEventsMetadata[1].Source)
		assert.Equal(t, events.PrefixEventType("loggedIn"), writtenEventsMetadata[1].Type)
		assert.Equal(t, "/users/loggedIn", writtenEventsMetadata[1].Subject)
		assert.Equal(t, "2", writtenEventsMetadata[1].ID)
		assert.Equal(t, johnLoggedIn.TracingContext, writtenEventsMetadata[1].TracingContext)

		assert.NoError(t, err)
	})

	t.Run("writes multiple events.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe
		johnRegistered := events.Events.Registered.JohnDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data, event.WithTracingContext(janeRegistered.TracingContext)),
				source.NewEvent(subject, johnRegistered.Type, johnRegistered.Data, event.WithTracingContext(johnRegistered.TracingContext)),
			},
		)
		assert.NoError(t, err)
	})

	t.Run("returns a sever error if the server responds with HTTP 5xx on every try", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/write-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token", eventsourcingdb.MaxTries(2))
		assert.NoError(t, err)

		source := event.NewSource(events.TestSource)
		subject := "/" + uuid.New().String()

		janeRegistered := events.Events.Registered.JaneDoe

		_, err = client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "retries exceeded")
		assert.ErrorContains(t, err, "Bad Gateway")
	})

	t.Run("returns an error if the server's protocol version does not match.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/write-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("X-EventSourcingDB-Protocol-Version", "0.0.0")
				writer.WriteHeader(http.StatusUnprocessableEntity)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		source := event.NewSource(events.TestSource)
		subject := "/" + uuid.New().String()

		janeRegistered := events.Events.Registered.JaneDoe

		_, err = client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrClientError))
		assert.ErrorContains(t, err, "client error\nprotocol version mismatch, server '0.0.0', client '1.0.0'")
	})

	t.Run("returns a client error if the server returns a 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/write-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadRequest)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		source := event.NewSource(events.TestSource)
		subject := "/" + uuid.New().String()

		janeRegistered := events.Events.Registered.JaneDoe

		_, err = client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrClientError))
		assert.ErrorContains(t, err, "Bad Request")
	})

	t.Run("returns a server error if the server returns a non 200, 5xx or 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/write-events", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		source := event.NewSource(events.TestSource)
		subject := "/" + uuid.New().String()

		janeRegistered := events.Events.Registered.JaneDoe

		_, err = client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "unexpected response status: 202 Accepted")
	})

	t.Run("returns a server error if the server's response can't be read.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/write-events", func(writer http.ResponseWriter, request *http.Request) {
				// Use an incorrect content length so the reader tries to read out of bounds.
				writer.Header().Set("Content-Length", "1")
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		source := event.NewSource(events.TestSource)
		subject := "/" + uuid.New().String()

		janeRegistered := events.Events.Registered.JaneDoe

		_, err = client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "failed to read the response body")
	})

	t.Run("returns a server error if the server's response can't be parsed.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/write-events", func(writer http.ResponseWriter, request *http.Request) {
				// Use an incorrect content length so the reader tries to read out of bounds.
				if _, err := writer.Write([]byte(":-)")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		source := event.NewSource(events.TestSource)
		subject := "/" + uuid.New().String()

		janeRegistered := events.Events.Registered.JaneDoe

		_, err = client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.Error(t, err)
		assert.True(t, errors.Is(err, customErrors.ErrServerError))
		assert.ErrorContains(t, err, "failed to parse the response body")
	})
}

func TestWriteEventsWithPreconditions(t *testing.T) {
	t.Run("when using the 'is subject pristine' precondition", func(t *testing.T) {
		t.Run("returns an error if the IsSubjectPristine precondition uses an invalid subject.", func(t *testing.T) {
			client := database.WithAuthorization.GetClient()

			_, err := client.WriteEvents(
				[]event.Candidate{
					event.NewCandidate("tag:foobar.com,2023:barbaz", "/foobar", "com.foobar.barbaz", struct{}{}),
				},
				eventsourcingdb.IsSubjectPristine("invalid"),
			)

			assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
			assert.ErrorContains(t, err, "parameter 'preconditions' is invalid\nIsSubjectPristine is invalid: malformed event subject 'invalid': subject must be an absolute, slash-separated path")
		})

		t.Run("writes events if the subject is pristine.", func(t *testing.T) {
			client := database.WithAuthorization.GetClient()
			source := event.NewSource(events.TestSource)

			subject := "/" + uuid.New().String()
			janeRegistered := events.Events.Registered.JaneDoe

			_, err := client.WriteEvents(
				[]event.Candidate{
					source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
				},
				eventsourcingdb.IsSubjectPristine(subject),
			)

			assert.NoError(t, err)
		})

		t.Run("returns an error if the subject is not pristine.", func(t *testing.T) {
			client := database.WithAuthorization.GetClient()
			source := event.NewSource(events.TestSource)

			subject := "/" + uuid.New().String()
			janeRegistered := events.Events.Registered.JaneDoe
			johnRegistered := events.Events.Registered.JohnDoe

			_, err := client.WriteEvents([]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			})

			assert.NoError(t, err)

			_, err = client.WriteEvents(
				[]event.Candidate{
					source.NewEvent(subject, johnRegistered.Type, johnRegistered.Data),
				},
				eventsourcingdb.IsSubjectPristine(subject),
			)

			assert.Error(t, err)
		})
	})

	t.Run("when using the 'is subject on event ID' precondition", func(t *testing.T) {
		t.Run("returns an error if the IsSubjectOnEventID precondition uses an invalid subject.", func(t *testing.T) {
			client := database.WithAuthorization.GetClient()

			_, err := client.WriteEvents(
				[]event.Candidate{
					event.NewCandidate("tag:foobar.com,2023:barbaz", "/foobar", "com.foobar.barbaz", struct{}{}),
				},
				eventsourcingdb.IsSubjectOnEventID("invalid", "123"),
			)

			assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
			assert.ErrorContains(t, err, "parameter 'preconditions' is invalid\nIsSubjectOnEventID is invalid: malformed event subject 'invalid': subject must be an absolute, slash-separated path")
		})

		t.Run("returns an error if the IsSubjectOnEventID precondition uses an eventID that does not contain an integer.", func(t *testing.T) {
			client := database.WithAuthorization.GetClient()

			_, err := client.WriteEvents(
				[]event.Candidate{
					event.NewCandidate("tag:foobar.com,2023:barbaz", "/foobar", "com.foobar.barbaz", struct{}{}),
				},
				eventsourcingdb.IsSubjectOnEventID("/", "borzel"),
			)

			assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
			assert.ErrorContains(t, err, "parameter 'preconditions' is invalid\nIsSubjectOnEventID is invalid: eventID must contain an integer")
		})

		t.Run("returns an error if the IsSubjectOnEventID precondition uses an eventID that contains a negative integer", func(t *testing.T) {
			client := database.WithAuthorization.GetClient()

			_, err := client.WriteEvents(
				[]event.Candidate{
					event.NewCandidate("tag:foobar.com,2023:barbaz", "/foobar", "com.foobar.barbaz", struct{}{}),
				},
				eventsourcingdb.IsSubjectOnEventID("/", "-1"),
			)

			assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
			assert.ErrorContains(t, err, "parameter 'preconditions' is invalid\nIsSubjectOnEventID is invalid: eventID must be 0 or greater")
		})

		t.Run("writes events if the last event of the subject has the given event ID.", func(t *testing.T) {
			client := database.WithAuthorization.GetClient()
			source := event.NewSource(events.TestSource)

			janeRegistered := events.Events.Registered.JaneDoe
			johnRegistered := events.Events.Registered.JohnDoe
			fredRegistered := events.Events.Registered.ApfelFred

			_, err := client.WriteEvents(
				[]event.Candidate{
					source.NewEvent("/users", janeRegistered.Type, janeRegistered.Data),
					source.NewEvent("/users", johnRegistered.Type, johnRegistered.Data),
				},
			)

			assert.NoError(t, err)

			readEvents := client.ReadEvents(context.Background(), "/users", eventsourcingdb.ReadNonRecursively())

			var lastEventID string
			for readEvent := range readEvents {
				data, err := readEvent.GetData()
				assert.NoError(t, err)

				lastEventID = data.Event.ID
			}

			_, err = client.WriteEvents(
				[]event.Candidate{
					source.NewEvent("/users", fredRegistered.Type, fredRegistered.Data),
				},
				eventsourcingdb.IsSubjectOnEventID("/users", lastEventID),
			)

			assert.NoError(t, err)
		})

		t.Run("returns an error if the last event of the subject does not have the given event ID.", func(t *testing.T) {
			client := database.WithAuthorization.GetClient()
			source := event.NewSource(events.TestSource)

			janeRegistered := events.Events.Registered.JaneDoe
			johnRegistered := events.Events.Registered.JohnDoe
			fredRegistered := events.Events.Registered.ApfelFred

			_, err := client.WriteEvents(
				[]event.Candidate{
					source.NewEvent("/users", janeRegistered.Type, janeRegistered.Data),
					source.NewEvent("/users", johnRegistered.Type, johnRegistered.Data),
				},
			)

			assert.NoError(t, err)

			lastEventID := "1337"

			_, err = client.WriteEvents(
				[]event.Candidate{
					source.NewEvent("/users", fredRegistered.Type, fredRegistered.Data),
				},
				eventsourcingdb.IsSubjectOnEventID("/users", lastEventID),
			)

			assert.Error(t, err)
		})
	})

	t.Run("Returns an error when any of the given events does not validate against the schema.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		err := client.RegisterEventSchema("com.sauer.kraut", `{"type":"object","additionalProperties":false}`)
		assert.NoError(t, err)

		_, err = client.WriteEvents([]event.Candidate{
			source.NewEvent("/knabberzeug", "com.sauer.kraut", map[string]string{"foo": "bar"}),
		})
		assert.ErrorContains(t, err, "409 Conflict")
	})
}
