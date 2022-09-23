package eventsourcingdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
)

func TestPing(t *testing.T) {
	t.Run("supports authorization.", func(t *testing.T) {
		client := eventsourcingdb.NewClientWithOptions(baseURLWithAuthorization, eventsourcingdb.ClientOptions{
			AccessToken: accessToken,
		})
		err := client.Ping()

		assert.NoError(t, err)
	})

	t.Run("returns nil if EventSourcingDB is reachable.", func(t *testing.T) {
		client := eventsourcingdb.NewClient(baseURLWithoutAuthorization)
		err := client.Ping()

		assert.NoError(t, err)
	})

	t.Run("returns an error if an invalid url is given.", func(t *testing.T) {
		client := eventsourcingdb.NewClient("http://localhost.invalid")
		err := client.Ping()

		assert.Error(t, err)
	})
}
