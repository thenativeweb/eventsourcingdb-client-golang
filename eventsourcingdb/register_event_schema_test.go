package eventsourcingdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestRegisterEventSchema(t *testing.T) {
	t.Run("registers an event schema", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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
		assert.NoError(t, err)
	})

	t.Run("returns an error if an event schema is already registered", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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

		err = client.RegisterEventSchema(
			eventType,
			schema,
		)
		assert.EqualError(t, err, "failed to register event schema, got HTTP status code '409', expected '200'")
	})
}
