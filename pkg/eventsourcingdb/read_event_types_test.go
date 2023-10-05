package eventsourcingdb_test

import (
	"context"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/goutils/v2/platformutils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
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

	// Regression test for https://github.com/thenativeweb/eventsourcingdb-client-golang/pull/97
	t.Run("Works with contexts that have a deadline.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*platformutils.Jiffy))
		defer cancel()

		time.Sleep(2 * platformutils.Jiffy)

		results := client.ReadEventTypes(ctx)
		result := <-results
		_, err := result.GetData()

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.NotErrorIs(t, customErrors.ErrServerError, err)
		assert.NotErrorIs(t, customErrors.ErrClientError, err)
		assert.NotErrorIs(t, customErrors.ErrInternalError, err)
		assert.NotContains(t, err.Error(), "unsupported stream item")
	})
}
