package eventsourcingdb_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
)

func TestNewClient(t *testing.T) {
	t.Run("returns an error if the baseURL is malformed.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("$%&/()", "access-token")

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'baseURL' is invalid\n")
	})

	t.Run("returns an error if the baseURL uses neither the HTTP scheme nor HTTPS scheme.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("telnet://foobar.invalid", "access-token")

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'baseURL' is invalid\nmust use HTTP or HTTPS")
	})

	t.Run("returns no error if the baseURL uses the HTTP scheme.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("http://foobar.invalid", "access-token")

		assert.NoError(t, err)
	})

	t.Run("returns no error if the baseURL uses the HTTPS scheme.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("https://foobar.invalid", "access-token")

		assert.NoError(t, err)
	})

	t.Run("returns an error if maxTries is less than 1.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("http://foobar.invalid", "access-token", eventsourcingdb.MaxTries(0))

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'MaxTries' is invalid\nmaxTries must be 1 or greater")
	})

	t.Run("returns an error if the accessToken is empty.", func(t *testing.T) {
		_, err := eventsourcingdb.NewClient("http://foobar.invalid", "")

		assert.True(t, errors.Is(err, customErrors.ErrInvalidParameter))
		assert.ErrorContains(t, err, "parameter 'AccessToken' is invalid\nthe access token must not be empty")
	})
}
