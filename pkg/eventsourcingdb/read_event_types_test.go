package eventsourcingdb_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"testing"
)

func TestClient_ReadEventTypes(t *testing.T) {
	t.Run("Reads all event types of existing events, as well as all event types with registered schemas.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		_, err := client.WriteEvents([]event.Candidate{
			source.NewEvent("/account", "com.foo.bar", map[string]string{}),
			source.NewEvent("/account/user", "com.bar.baz", map[string]string{}),
			source.NewEvent("/account/user", "com.baz.leml", map[string]string{}),
			source.NewEvent("/", "com.quux.knax", map[string]string{}),
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
}
