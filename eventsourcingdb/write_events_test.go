package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
)

func TestWriteEvents(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("writes a single event", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				event,
			},
			nil,
		)
		assert.NoError(t, err)
		assert.Len(t, writtenEvents, 1)
		assert.Equal(t, "0", writtenEvents[0].ID)
	})

	t.Run("writes the trace context of an event", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		traceParent := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
		traceState := "rojo=00f067aa0ba902b7"

		event := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
			TraceParent: internal.Ptr(traceParent),
			TraceState:  internal.Ptr(traceState),
		}

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				event,
			},
			nil,
		)
		require.NoError(t, err)
		require.Len(t, writtenEvents, 1)
		assert.Equal(t, &traceParent, writtenEvents[0].TraceParent)
		assert.Equal(t, &traceState, writtenEvents[0].TraceState)

		eventsRead := []eventsourcingdb.Event{}
		for event, err := range client.ReadEvents(
			ctx,
			"/test",
			eventsourcingdb.ReadEventsOptions{
				Recursive: false,
			},
		) {
			require.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		require.Len(t, eventsRead, 1)
		assert.Equal(t, &traceParent, eventsRead[0].TraceParent)
		assert.Equal(t, &traceState, eventsRead[0].TraceState)
	})

	t.Run("writes multiple events", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		assert.NoError(t, err)
		assert.Len(t, writtenEvents, 2)

		var eventData EventData

		assert.Equal(t, "0", writtenEvents[0].ID)
		err = json.Unmarshal(writtenEvents[0].Data, &eventData)
		assert.NoError(t, err)
		assert.Equal(t, 23, eventData.Value)

		assert.Equal(t, "1", writtenEvents[1].ID)
		err = json.Unmarshal(writtenEvents[1].Data, &eventData)
		assert.NoError(t, err)
		assert.Equal(t, 42, eventData.Value)
	})

	t.Run("supports the isSubjectPristine precondition", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectPristinePrecondition("/test"),
			},
		)

		assertReasonOfServer(t, err, "write events", http.StatusConflict, "state conflict: ")
	})

	t.Run("supports the isSubjectPopulated precondition", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test"),
			},
		)
		assertReasonOfServer(t, err, "write events", http.StatusConflict, "state conflict: ")

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		writtenEvents, err := client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test"),
			},
		)
		assert.NoError(t, err)
		assert.Len(t, writtenEvents, 1)
		assert.Equal(t, "1", writtenEvents[0].ID)

		var eventData EventData
		err = json.Unmarshal(writtenEvents[0].Data, &eventData)
		assert.NoError(t, err)
		assert.Equal(t, 42, eventData.Value)
	})

	t.Run("supports the isSubjectOnEventId precondition", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "1"),
			},
		)

		assertReasonOfServer(t, err, "write events", http.StatusConflict, "state conflict: ")
	})

	t.Run("supports the isEventQlQueryTrue precondition", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsEventQLQueryTruePrecondition("FROM e IN events PROJECT INTO COUNT() == 0"),
			},
		)

		assertReasonOfServer(t, err, "write events", http.StatusConflict, "state conflict: ")
	})

	t.Run("reports the reason if an event does not match its schema", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		err = client.RegisterEventSchema(
			ctx,
			"io.eventsourcingdb.test",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"value": map[string]any{
						"type": "number",
					},
				},
				"required":             []string{"value"},
				"additionalProperties": false,
			},
		)
		require.NoError(t, err)

		_, err = client.WriteEvents(
			ctx,
			[]eventsourcingdb.EventCandidate{
				{
					Source:  "https://www.eventsourcingdb.io",
					Subject: "/test",
					Type:    "io.eventsourcingdb.test",
					Data: map[string]any{
						"value": 23,
						"extra": true,
					},
				},
			},
			nil,
		)

		assertReasonOfServer(t, err, "write events", http.StatusConflict, "schema conflict: ", "additionalProperties 'extra' not allowed")
	})
}
