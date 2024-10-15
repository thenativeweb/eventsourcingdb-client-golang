package eventsourcingdb_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
)

func TestNewClient(t *testing.T) {
	t.Run("returns an error if the baseURL is malformed.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("$%&/()", "access-token")

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'baseURL' is invalid:")
	})

	t.Run("returns an error if the baseURL uses neither the HTTP scheme nor HTTPS scheme.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("telnet://foobar.invalid", "access-token")

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'baseURL' is invalid: must use HTTP or HTTPS")
	})

	t.Run("returns no error if the baseURL uses the HTTP scheme.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("http://foobar.invalid", "access-token")

		assert.NoError(t, err)
	})

	t.Run("returns no error if the baseURL uses the HTTPS scheme.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("https://foobar.invalid", "access-token")

		assert.NoError(t, err)
	})

	t.Run("returns an error if the accessToken is empty.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("http://foobar.invalid", "")

		assert.True(t, errors.Is(err, eventsourcingdb.ErrInvalidArgument))
		assert.ErrorContains(t, err, "parameter 'AccessToken' is invalid: the access token must not be empty")
	})
}
