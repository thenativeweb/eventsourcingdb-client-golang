package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
)

func TestReadEventTypes(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("reads no event types if the database is empty", func(t *testing.T) {
		ctx := context.Background()

		container := eventsourcingdb.NewContainer()
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		didReadEventTypes := false

		for _, err := range client.ReadEventTypes(ctx) {
			assert.NoError(t, err)
			didReadEventTypes = true
		}

		assert.False(t, didReadEventTypes)
	})

	t.Run("reads all event types", func(t *testing.T) {
		ctx := context.Background()

		container := eventsourcingdb.NewContainer()
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test.foo",
			Data: EventData{
				Value: 23,
			},
		}

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test.bar",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		require.NoError(t, err)

		eventTypesRead := []eventsourcingdb.EventType{}

		for eventType, err := range client.ReadEventTypes(ctx) {
			assert.NoError(t, err)
			eventTypesRead = append(eventTypesRead, eventType)
		}

		assert.Len(t, eventTypesRead, 2)
		assert.Equal(t, eventTypesRead[0], eventsourcingdb.EventType{
			EventType: "io.eventsourcingdb.test.bar",
			IsPhantom: false,
		})
		assert.Equal(t, eventTypesRead[1], eventsourcingdb.EventType{
			EventType: "io.eventsourcingdb.test.foo",
			IsPhantom: false,
		})
	})

	t.Run("supports reading event schemas", func(t *testing.T) {
		ctx := context.Background()

		container := eventsourcingdb.NewContainer()
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		eventType := "io.eventsourcingdb.test"
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{
					"type": "number",
				},
			},
			"required":             []string{"value"},
			"additionalProperties": false,
		}

		err = client.RegisterEventSchema(
			eventType,
			schema,
		)
		require.NoError(t, err)

		eventTypesRead := []eventsourcingdb.EventType{}

		for eventType, err := range client.ReadEventTypes(ctx) {
			assert.NoError(t, err)
			eventTypesRead = append(eventTypesRead, eventType)
		}

		assert.Len(t, eventTypesRead, 1)
		assert.Equal(t, "io.eventsourcingdb.test", eventTypesRead[0].EventType)
		assert.True(t, eventTypesRead[0].IsPhantom)
		assert.NotNil(t, eventTypesRead[0].Schema)

		expectedJSON, err := json.Marshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{
					"type": "number",
				},
			},
			"required":             []string{"value"},
			"additionalProperties": false,
		})
		assert.NoError(t, err)

		actualJSON, err := json.Marshal(eventTypesRead[0].Schema)
		assert.NoError(t, err)

		assert.JSONEq(t, string(expectedJSON), string(actualJSON))
	})
}
