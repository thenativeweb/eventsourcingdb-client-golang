package eventsourcingdb_test

import (
	"context"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/httpserver"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
)

func TestReadSubjects(t *testing.T) {
	t.Run("returns a channel containing an error when trying to read from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()

		readSubjectResults := client.ReadSubjects(context.Background())

		_, err := (<-readSubjectResults).GetData()
		assert.True(t, customErrors.IsServerError(err))
		assert.ErrorContains(t, err, "server error: retries exceeded")
	})

	t.Run("closes the channel when no more subjects exist.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		readSubjectResults := client.ReadSubjects(context.Background())

		_, ok := <-readSubjectResults
		assert.False(t, ok)
	})

	t.Run("reads all subjects starting from /.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]event.Candidate{
			event.NewCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
		})

		assert.NoError(t, err)

		readSubjectResults := client.ReadSubjects(context.Background())
		subjects := make([]string, 0, 2)

		for result := range readSubjectResults {
			data, err := result.GetData()
			assert.NoError(t, err)

			subjects = append(subjects, data)
		}

		assert.Equal(t, []string{"/", subject}, subjects)
	})

	t.Run("reads subjects starting from the given base subject.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		subject := "/foobar/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]event.Candidate{
			event.NewCandidate(events.TestSource, subject, janeRegistered.Type, janeRegistered.Data),
		})

		assert.NoError(t, err)

		readSubjectResults := client.ReadSubjects(context.Background(), eventsourcingdb.BaseSubject("/foobar"))
		subjects := make([]string, 0, 2)

		for result := range readSubjectResults {
			data, err := result.GetData()
			assert.NoError(t, err)

			subjects = append(subjects, data)
		}

		assert.Equal(t, []string{"/foobar", subject}, subjects)
	})

	t.Run("returns an error if the context is cancelled before the request is sent.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		readSubjectResults := client.ReadSubjects(ctx)

		canceledResult := <-readSubjectResults
		_, err := canceledResult.GetData()
		assert.Error(t, err)
		assert.True(t, customErrors.IsContextCanceledError(err))

		superfluousResult, ok := <-readSubjectResults
		assert.False(t, ok, fmt.Sprintf("channel did not close %+v", superfluousResult))
	})

	t.Run("returns an error if the context is cancelled while reading the ndjson stream.", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte("{\"type\": \"item\", \"payload\": {}}\n")); err != nil {
					panic(err)
				}
				cancel()
			})
		})
		defer stopServer()

		client, _ := eventsourcingdb.NewClient(serverAddress, "access-token")
		resultChan := client.ReadSubjects(ctx)

		_, err := (<-resultChan).GetData()
		assert.Error(t, err)
		assert.True(t, customErrors.IsContextCanceledError(err), err.Error())
	})

	t.Run("returns an error when the base subject is malformed.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		results := client.ReadSubjects(context.Background(), eventsourcingdb.BaseSubject("schkibididopdop"))
		result := <-results

		_, err := result.GetData()
		assert.True(t, customErrors.IsInvalidParameterError(err))
		assert.ErrorContains(t, err, "parameter 'BaseSubject' is invalid: malformed event subject")
	})

	t.Run("returns a sever error if the server responds with HTTP 5xx on every try", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token", eventsourcingdb.MaxTries(2))
		assert.NoError(t, err)

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsServerError(err))
		assert.ErrorContains(t, err, "retries exceeded")
		assert.ErrorContains(t, err, "Bad Gateway")
	})

	t.Run("returns an error if the server's protocol version does not match.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("X-EventSourcingDB-Protocol-Version", "0.0.0")
				writer.WriteHeader(http.StatusUnprocessableEntity)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsClientError(err))
		assert.ErrorContains(t, err, "client error: protocol version mismatch, server '0.0.0', client '1.0.0'")
	})

	t.Run("returns a client error if the server returns a 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadRequest)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsClientError(err))
		assert.ErrorContains(t, err, "Bad Request")
	})

	t.Run("returns a server error if the server returns a non 200, 5xx or 4xx status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/api/read-subjects", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsServerError(err))
		assert.ErrorContains(t, err, "unexpected response status: 202 Accepted")
	})

	t.Run("returns a server error if the server sends a stream item that can't be unmarshalled.", func(t *testing.T) {
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

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsServerError(err))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered: cannot unmarshal")
	})

	t.Run("returns a server error if the server sends a stream item that can't be unmarshalled.", func(t *testing.T) {
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

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsServerError(err))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered:")
		assert.ErrorContains(t, err, "does not have a recognized type")
	})

	t.Run("returns a server error if the server sends a an error item through the ndjson stream.", func(t *testing.T) {
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

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsServerError(err))
		assert.ErrorContains(t, err, "server error: aliens have abducted the server")
	})

	t.Run("returns a server error if the server sends a an error item through the ndjson stream, but the error can't be unmarshalled.", func(t *testing.T) {
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

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsServerError(err))
		assert.ErrorContains(t, err, "server error: unsupported stream error encountered:")
	})

	t.Run("returns a server error if the server sends an item that can't be unmarshalled.", func(t *testing.T) {
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

		results := client.ReadSubjects(context.Background())
		result := <-results
		_, err = result.GetData()

		assert.Error(t, err)
		assert.True(t, customErrors.IsServerError(err))
		assert.ErrorContains(t, err, "server error: unsupported stream item encountered:")
		assert.ErrorContains(t, err, "(trying to unmarshal")
	})
}
