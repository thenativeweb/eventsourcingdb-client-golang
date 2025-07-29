package eventsourcingdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestReadEventType(t *testing.T) {
	t.Run("fails if the event type does not exist", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(t.Context())
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		_, err = client.ReadEventType("io.eventsourcingdb.test.nonexistent")
		require.Error(t, err)
		assert.Equal(t, "failed to read event type, got HTTP status code '404', expected '200'", err.Error())
	})

	t.Run("fails if the event type is malformed", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(t.Context())
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		_, err = client.ReadEventType("io.eventsourcingdb.test.")
		require.Error(t, err)
		assert.Equal(t, "failed to read event type, got HTTP status code '400', expected '200'", err.Error())
	})

	t.Run("reads an existing event type", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(t.Context())
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.RegisterEventSchema("io.eventsourcingdb.test.foo", map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		})
		require.NoError(t, err)

		eventType, err := client.ReadEventType("io.eventsourcingdb.test.foo")
		require.NoError(t, err)

		assert.Equal(t, "io.eventsourcingdb.test.foo", eventType.EventType)
		assert.True(t, eventType.IsPhantom)
		assert.Equal(t, &map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}, eventType.Schema)
	})
}
