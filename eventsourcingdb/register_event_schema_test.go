package eventsourcingdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
)

func TestRegisterEventSchema(t *testing.T) {
	t.Run("registers an event schema", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
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
			ctx,
			eventType,
			schema,
		)
		assert.NoError(t, err)
	})

	t.Run("returns an error if an event schema is already registered", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
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
			ctx,
			eventType,
			schema,
		)
		require.NoError(t, err)

		err = client.RegisterEventSchema(
			ctx,
			eventType,
			schema,
		)
		assertCodeOfServer(t, err, "register event schema", eventsourcingdb.CodeEventSchemaAlreadyRegistered)
	})
	t.Run("returns an error if an event that was written before does not match the schema", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		_, err = client.WriteEvents(ctx, []eventsourcingdb.EventCandidate{
			{
				Source:  "https://www.eventsourcingdb.io",
				Subject: "/test",
				Type:    "io.eventsourcingdb.test",
				Data:    map[string]any{"value": 23},
			},
		}, nil)
		require.NoError(t, err)

		err = client.RegisterEventSchema(
			ctx,
			"io.eventsourcingdb.test",
			map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{"title"},
			},
		)
		assertCodeOfServer(t, err, "register event schema", eventsourcingdb.CodeEventSchemaViolated)
	})
}
