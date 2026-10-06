package eventsourcingdb_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
)

// A caller has to be able to tell a server that is not EventSourcingDB, e.g. a
// proxy that answers in place of the database, from the database itself.
func TestErrInvalidServerHeader(t *testing.T) {
	t.Run("every request reports a server without a Server header", func(t *testing.T) {
		client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		for _, request := range requestsOf(client) {
			t.Run(request.name, func(t *testing.T) {
				err := request.send()

				assert.ErrorIs(t, err, eventsourcingdb.ErrInvalidServerHeader)
				assert.EqualError(t, err, "server must be EventSourcingDB")
			})
		}
	})

	t.Run("every request reports another server instead of its status", func(t *testing.T) {
		client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "nginx/1.29.0")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad gateway"))
		}))

		for _, request := range requestsOf(client) {
			t.Run(request.name, func(t *testing.T) {
				err := request.send()

				var dbAPIError *eventsourcingdb.DBAPIError
				assert.ErrorIs(t, err, eventsourcingdb.ErrInvalidServerHeader)
				assert.NotErrorAs(t, err, &dbAPIError)
				assert.EqualError(t, err, "server must be EventSourcingDB")
			})
		}
	})

	t.Run("is found if it is wrapped", func(t *testing.T) {
		client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		err := client.Ping(context.Background())
		wrapped := fmt.Errorf("failed to handle the command: %w", err)

		assert.ErrorIs(t, wrapped, eventsourcingdb.ErrInvalidServerHeader)
	})
}
