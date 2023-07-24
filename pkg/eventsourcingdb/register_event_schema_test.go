package eventsourcingdb_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"testing"
)

func TestClient_RegisterEventSchema(t *testing.T) {
	t.Run("Registers the new schema if it doesn't conflict with existing events.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		err := client.RegisterEventSchema("com.ekht.ekht", `{"type": "object"}`)
		assert.NoError(t, err)
	})

	t.Run("Rejects the request if at least one of the existing events conflicts with the schema.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		_, err := client.WriteEvents([]event.Candidate{
			event.NewCandidate(events.TestSource, "/", "com.gornisht.ekht", map[string]string{"oy": "gevalt"}),
		})
		assert.NoError(t, err)

		err = client.RegisterEventSchema("com.gornisht.ekht", `{"type": "object", "additionalProperties": false}`)
		assert.ErrorContains(t, err, "client error: Conflict: schema conflicts with existing event (ID=0)")
	})

	t.Run("Rejects the request if the schema already exists.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		err := client.RegisterEventSchema("com.ekht.ekht", `{"type": "object"}`)
		assert.NoError(t, err)

		err = client.RegisterEventSchema("com.ekht.ekht", `{"type": "object"}`)
		assert.ErrorContains(t, err, "client error: Conflict: schema already exists")
	})

	t.Run("Rejects the request if the given schema is invalid.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		err := client.RegisterEventSchema("com.ekht.ekht", `{"type": `)
		assert.ErrorContains(t, err, "Bad Request: schema conflicts with existing event (ID=0)")
	})
}
