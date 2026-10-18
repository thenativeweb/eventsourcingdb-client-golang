package eventsourcingdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
)

func TestRegisterEventSchema(t *testing.T) {
	t.Run("registers an event schema", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
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

		imageTag, err := internal.GetImageTag()
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
}
