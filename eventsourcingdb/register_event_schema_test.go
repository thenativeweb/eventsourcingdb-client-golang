package eventsourcingdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb/event"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
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
			event.NewCandidate(events.TestSource, "/", "com.gornisht.ekht", map[string]any{"oy": "gevalt"}),
		})
		assert.NoError(t, err)

		err = client.RegisterEventSchema("com.gornisht.ekht", `{"type": "object", "additionalProperties": false}`)
		assert.ErrorContains(t, err, "schema conflict: event with ID 0: additionalProperties 'oy' not allowed")
	})

	t.Run("Rejects the request if the schema already exists.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		err := client.RegisterEventSchema("com.ekht.ekht", `{"type": "object"}`)
		assert.NoError(t, err)

		err = client.RegisterEventSchema("com.ekht.ekht", `{"type": "object"}`)
		assert.ErrorContains(t, err, "schema conflict: schema already exists")
	})

	t.Run("Rejects the request if the given schema is invalid.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		err := client.RegisterEventSchema("com.ekht.ekht", `{"type": `)
		assert.ErrorContains(t, err, "jsonschema: invalid json")
	})
}
