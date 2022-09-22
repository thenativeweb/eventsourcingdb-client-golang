package eventsourcingdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
)

func TestPing(t *testing.T) {
	t.Run("Returns nil when the database is reachable.", func(t *testing.T) {
		client := eventsourcingdb.NewClient(baseUrl)
		err := client.Ping()

		assert.NoError(t, err)
	})
	t.Run("Returns ErrPingFailed when the database is not reachable.", func(t *testing.T) {
		client := eventsourcingdb.NewClient("http://lokalhorst.invalid")
		err := client.Ping()

		assert.Error(t, err)
	})
}
