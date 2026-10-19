package eventsourcingdb_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
)

func TestReadEventType(t *testing.T) {
	t.Run("fails if the event type does not exist", func(t *testing.T) {
		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(t.Context())
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		_, err = client.ReadEventType(t.Context(), "io.eventsourcingdb.test.nonexistent")
		require.Error(t, err)
		assertCodeOfServer(t, err, "read event type", eventsourcingdb.CodeEventTypeUnknown)
	})

	t.Run("fails if the event type is malformed", func(t *testing.T) {
		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(t.Context())
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		_, err = client.ReadEventType(t.Context(), "io.eventsourcingdb.test.")
		require.Error(t, err)
		assert.ErrorContains(t, err, "event type")

		// The v1 API refuses the request as malformed, while the v2 API
		// tells that the event type is.
		if eventsourcingdb.APIOf(client) == "v2" {
			assertCodeOfServer(t, err, "read event type", eventsourcingdb.CodeEventTypeMalformed)
			assertDBAPIError(t, err, http.StatusOK, eventsourcingdb.CodeEventTypeMalformed, "event type is malformed: expected a reverse domain name such as 'io.eventsourcingdb.library.book-acquired'")
		} else {
			assertCodeOfServer(t, err, "read event type", "")
			assertDBAPIError(t, err, http.StatusBadRequest, "", "invalid event type: 'io.eventsourcingdb.test.'")
		}
	})

	t.Run("reads an existing event type", func(t *testing.T) {
		imageTag, err := getImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(t.Context())
		defer container.Stop(t.Context())

		client, err := container.GetClient(t.Context())
		require.NoError(t, err)

		err = client.RegisterEventSchema(t.Context(), "io.eventsourcingdb.test.foo", map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		})
		require.NoError(t, err)

		eventType, err := client.ReadEventType(t.Context(), "io.eventsourcingdb.test.foo")
		require.NoError(t, err)

		assert.Equal(t, "io.eventsourcingdb.test.foo", eventType.Type)
		assert.False(t, eventType.HasEvents)
		assert.Equal(t, &map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}, eventType.Schema)
	})
}
