package eventsourcingdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
)

func TestClient_ReadEventTypes(t *testing.T) {
	t.Run("Reads all event types of existing events, as well as all event types with registered schemas.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		source := eventsourcingdb.NewSource(events.TestSource)

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			source.NewEvent("/account", "com.foo.bar", map[string]any{}),
			source.NewEvent("/account/user", "com.bar.baz", map[string]any{}),
			source.NewEvent("/account/user", "com.baz.leml", map[string]any{}),
			source.NewEvent("/", "com.quux.knax", map[string]any{}),
		})
		assert.NoError(t, err)

		err = client.RegisterEventSchema("org.ban.ban", `{"type":"object"}`)
		assert.NoError(t, err)

		err = client.RegisterEventSchema("org.bing.chilling", `{"type":"object"}`)
		assert.NoError(t, err)

		results := client.ReadEventTypes(context.Background())
		expectedResults := []eventsourcingdb.EventType{
			{
				EventType: "com.foo.bar",
				IsPhantom: false,
				Schema:    "",
			},
			{
				EventType: "com.bar.baz",
				IsPhantom: false,
				Schema:    "",
			},
			{
				EventType: "com.baz.leml",
				IsPhantom: false,
				Schema:    "",
			},
			{
				EventType: "com.quux.knax",
				IsPhantom: false,
				Schema:    "",
			},
			{
				EventType: "org.ban.ban",
				IsPhantom: true,
				Schema:    `{"type":"object"}`,
			},
			{
				EventType: "org.bing.chilling",
				IsPhantom: true,
				Schema:    `{"type":"object"}`,
			},
		}

		var observedEventTypes []eventsourcingdb.EventType
		for result := range results {
			data, err := result.GetData()
			assert.NoError(t, err)
			if err != nil {
				continue
			}

			observedEventTypes = append(observedEventTypes, data)
		}

		for _, expectedEventType := range expectedResults {
			assert.Contains(t, observedEventTypes, expectedEventType)
		}
	})

	// Regression test for https://github.com/thenativeweb/eventsourcingdb-client-golang/pull/97
	t.Run("Works with contexts that have a deadline.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Millisecond))
		defer cancel()

		time.Sleep(2 * time.Millisecond)

		results := client.ReadEventTypes(ctx)
		result := <-results
		_, err := result.GetData()

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.NotErrorIs(t, eventsourcingdb.ErrServerError, err)
		assert.NotErrorIs(t, eventsourcingdb.ErrClientError, err)
		assert.NotErrorIs(t, eventsourcingdb.ErrInternalError, err)
		assert.NotContains(t, err.Error(), "unsupported stream item")
	})
}
