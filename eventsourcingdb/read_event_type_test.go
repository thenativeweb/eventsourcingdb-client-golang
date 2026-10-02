package eventsourcingdb_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestReadEventType(t *testing.T) {
	t.Run("fails if the event type does not exist", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)
		container.Start(t.Context())
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		_, err = client.ReadEventType("io.eventsourcingdb.test.nonexistent")
		require.Error(t, err)
		assert.Equal(t, "failed to read event type, got HTTP status code '404', expected '200': event type 'io.eventsourcingdb.test.nonexistent' not found", err.Error())
		assertDBAPIError(t, err, http.StatusNotFound, "event type 'io.eventsourcingdb.test.nonexistent' not found")
	})

	t.Run("fails if the event type is malformed", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)
		container.Start(t.Context())
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		_, err = client.ReadEventType("io.eventsourcingdb.test.")
		require.Error(t, err)
		assert.Equal(t, "failed to read event type, got HTTP status code '400', expected '200': invalid event type: 'io.eventsourcingdb.test.'", err.Error())
		assertDBAPIError(t, err, http.StatusBadRequest, "invalid event type: 'io.eventsourcingdb.test.'")
	})

	t.Run("reads an existing event type", func(t *testing.T) {
		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)
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
