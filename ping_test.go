package eventsourcingdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPing(t *testing.T) {
	t.Run("returns an error if an invalid url is given.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()
		err := client.Ping()

		assert.Error(t, err)
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		err := client.Ping()

		assert.NoError(t, err)
	})

	t.Run("does not return an error if EventSourcingDB is reachable.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()
		err := client.Ping()

		assert.NoError(t, err)
	})
}
