package eventsourcingdb_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/httpserver"
)

func TestPing(t *testing.T) {
	t.Run("returns an error if the server is not reachable.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()
		err := client.Ping()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "server did not respond")
	})

	t.Run("returns an error if the server responds with an unexpected status code.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/ping", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		err = client.Ping()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "server responded with an unexpected status: 502 Bad Gateway")
	})

	t.Run("returns an error if the server's response body can't be read.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/ping", func(writer http.ResponseWriter, request *http.Request) {
				// Set an incorrect content length so that the reader tries to read out of bounds.
				writer.Header().Set("Content-Length", "1")
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		err = client.Ping()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "failed to read response body")
	})

	t.Run("returns an error if the server's response body is not 'OK'.", func(t *testing.T) {
		serverAddress, stopServer := httpserver.NewHTTPServer(func(mux *http.ServeMux) {
			mux.HandleFunc("/ping", func(writer http.ResponseWriter, request *http.Request) {
				if _, err := writer.Write([]byte(":-)")); err != nil {
					panic(err)
				}
			})
		})
		defer stopServer()

		client, err := eventsourcingdb.NewClient(serverAddress, "access-token")
		assert.NoError(t, err)

		err = client.Ping()

		assert.True(t, errors.Is(err, eventsourcingdb.ErrServerError))
		assert.ErrorContains(t, err, "server responded with an unexpected response body: :-)")
	})

	t.Run("does not return an error if EventSourcingDB is reachable.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		err := client.Ping()

		assert.NoError(t, err)
	})
}
