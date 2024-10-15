package eventsourcingdb_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/httpserver"
)

func TestReadSubjects(t *testing.T) {
	t.Run("returns an iterator yielding exactly one server error when trying to read from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			count += 1
			assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		}

		assert.Equal(t, 1, count)
	})

	t.Run("closes iterator when no more subjects exist.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		for _, err := range client.ReadSubjects(context.Background()) {
			assert.NoError(t, err)
		}
	})

	t.Run("reads all subjects starting from /.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
		})
		assert.NoError(t, err)

		subjects := make([]string, 0, 2)
		for subject, err := range client.ReadSubjects(context.Background()) {
			assert.NoError(t, err)

			subjects = append(subjects, subject)
		}

		assert.Equal(t, []string{"/", subject}, subjects)
	})

	t.Run("reads subjects starting from the given base subject.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		subject := "/foobar/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
		})
		assert.NoError(t, err)

		subjects := make([]string, 0, 2)
		for subject, err := range client.ReadSubjects(context.Background(), eventsourcingdb.BaseSubject("/foobar")) {
			assert.NoError(t, err)

			subjects = append(subjects, subject)
		}

		assert.Equal(t, []string{"/foobar", subject}, subjects)
	})

	t.Run("yields exactly one error if the context is cancelled before the request is sent.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		count := 0
		for _, err := range client.ReadSubjects(ctx) {
			count++
			assert.ErrorIs(t, err, context.Canceled)
		}

		assert.Equal(t, 1, count)
	})

	t.Run("returns an error if the context is cancelled while reading the ndjson stream.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
			eventsourcingdb.NewEventCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
			eventsourcingdb.NewEventCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
			eventsourcingdb.NewEventCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
			eventsourcingdb.NewEventCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
		})
		assert.NoError(t, err)

		count := 0
		for _, err = range client.ReadSubjects(ctx) {
			if count == 0 {
				assert.NoError(t, err)
				cancel()
			}
			count += 1
		}

		assert.GreaterOrEqual(t, count, 2)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("yields exactly one error when the base subject is malformed.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		count := 0
		for _, err := range client.ReadSubjects(context.Background(), eventsourcingdb.BaseSubject("schkibididopdop")) {
			assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
			assert.ErrorContains(t, err, "argument 'BaseSubject' is invalid: malformed event subject")
			count += 1
		}

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one error if the server responds with HTTP 5xx", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
			assert.ErrorContains(t, err, "Bad Gateway")
			count += 1
		}

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one error if the server's protocol version does not match.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("X-EventSourcingDB-Protocol-Version", "0.0.0")
				writer.WriteHeader(http.StatusUnprocessableEntity)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrClientError))
			assert.ErrorContains(t, err, "client error: protocol version mismatch, server '0.0.0', client '1.0.0'")
		}
		count += 1

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one client error if the server returns a 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadRequest)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrClientError))
			assert.ErrorContains(t, err, "Bad Request")
			count += 1
		}

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one server error if the server returns a non 200, 5xx or 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
			assert.ErrorContains(t, err, "unexpected response status: 202 Accepted")
			count += 1
		}

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one server error if the server sends a stream item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": 42}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
			assert.ErrorContains(t, err, "server error: unsupported stream item encountered: cannot unmarshal")
			count += 1
		}

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one server error if the server sends a stream item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"clowns\": 8}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
			assert.ErrorContains(t, err, "server error: unsupported stream item encountered:")
			assert.ErrorContains(t, err, "does not have a recognized type")
			count += 1
		}

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one server error if the server sends a an error item through the ndjson stream.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"error\", \"payload\": {\"error\": \"aliens have abducted the server\"}}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
			assert.ErrorContains(t, err, "server error: aliens have abducted the server")
			count++
		}

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one a server error if the server sends a an error item through the ndjson stream, but the error can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"error\", \"payload\": {\"error\": 8}}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
			assert.ErrorContains(t, err, "server error: unsupported stream error encountered:")
			count += 1
		}

		assert.Equal(t, 1, count)
	})

	t.Run("yields exactly one server error if the server sends an item that can't be unmarshalled.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"subject\", \"subject\": 8}\n")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		count := 0
		for _, err := range client.ReadSubjects(context.Background()) {
			assert.Error(t, err)
			assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
			assert.ErrorContains(t, err, "server error: unsupported stream item encountered:")
			assert.ErrorContains(t, err, "(trying to unmarshal")
			count += 1
		}

		assert.Equal(t, 1, count)
	})

	// Regression test for https://github.com/thenativeweb/eventsourcingdb-client-golang/pull/97
	t.Run("Works with contexts that have a deadline.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Millisecond))
		defer cancel()

		time.Sleep(2 * time.Millisecond)

		count := 0
		for _, err := range client.ReadSubjects(ctx) {
			assert.ErrorIs(t, err, context.DeadlineExceeded)
			assert.NotErrorIs(t, eventsourcingdb.ErrServerError, err)
			assert.NotErrorIs(t, eventsourcingdb.ErrClientError, err)
			assert.NotErrorIs(t, eventsourcingdb.ErrInternalError, err)
			assert.NotContains(t, err.Error(), "unsupported stream item")
			count += 1
		}

		assert.Equal(t, 1, count)
	})
}
