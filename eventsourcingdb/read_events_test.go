package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestReadEvents(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("reads no events if the database is empty", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		didReadEvents := false

		for _, err := range client.ReadEvents(
			ctx,
			"/",
			eventsourcingdb.ReadEventsOptions{
				Recursive: true,
			},
		) {
			assert.NoError(t, err)
			didReadEvents = true
		}

		assert.False(t, didReadEvents)
	})

	t.Run("reads all events from the given subject", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		require.NoError(t, err)

		eventsRead := []eventsourcingdb.Event{}

		for event, err := range client.ReadEvents(
			ctx,
			"/test",
			eventsourcingdb.ReadEventsOptions{
				Recursive: false,
			},
		) {
			assert.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		assert.Len(t, eventsRead, 2)
	})

	t.Run("reads a huge event without truncating its payload", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		// Use characters that require JSON escaping so the serialized line is
		// significantly larger than the raw payload size.
		hugeValue := strings.Repeat(`"\`, 32*1024)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test",
			Data: map[string]any{
				"value": hugeValue,
			},
		}

		_, err = client.WriteEvents(
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

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

		var data struct {
			Value string `json:"value"`
		}
		err = json.Unmarshal(eventsRead[0].Data, &data)
		require.NoError(t, err)

		// Verify the payload is complete and identical to what was written,
		// not just that reading succeeded.
		assert.Len(t, data.Value, len(hugeValue))
		assert.Equal(t, hugeValue, data.Value)
	})

	t.Run("reads recursively", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		require.NoError(t, err)

		eventsRead := []eventsourcingdb.Event{}

		for event, err := range client.ReadEvents(
			ctx,
			"/",
			eventsourcingdb.ReadEventsOptions{
				Recursive: true,
			},
		) {
			assert.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		assert.Len(t, eventsRead, 2)
	})

	t.Run("reads chronologically", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		require.NoError(t, err)

		eventsRead := []eventsourcingdb.Event{}

		for event, err := range client.ReadEvents(
			ctx,
			"/test",
			eventsourcingdb.ReadEventsOptions{
				Recursive: false,
				Order:     eventsourcingdb.OrderChronological(),
			},
		) {
			assert.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		assert.Len(t, eventsRead, 2)

		var firstData EventData
		err = json.Unmarshal(eventsRead[0].Data, &firstData)
		require.NoError(t, err)
		assert.Equal(t, 23, firstData.Value)

		var secondData EventData
		err = json.Unmarshal(eventsRead[1].Data, &secondData)
		require.NoError(t, err)
		assert.Equal(t, 42, secondData.Value)
	})

	t.Run("reads antichronologically", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		require.NoError(t, err)

		eventsRead := []eventsourcingdb.Event{}

		for event, err := range client.ReadEvents(
			ctx,
			"/test",
			eventsourcingdb.ReadEventsOptions{
				Recursive: false,
				Order:     eventsourcingdb.OrderAntichronological(),
			},
		) {
			assert.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		assert.Len(t, eventsRead, 2)

		var firstData EventData
		err = json.Unmarshal(eventsRead[0].Data, &firstData)
		require.NoError(t, err)
		assert.Equal(t, 42, firstData.Value)

		var secondData EventData
		err = json.Unmarshal(eventsRead[1].Data, &secondData)
		require.NoError(t, err)
		assert.Equal(t, 23, secondData.Value)
	})

	t.Run("reads with lower bound", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		require.NoError(t, err)

		eventsRead := []eventsourcingdb.Event{}

		for event, err := range client.ReadEvents(
			ctx,
			"/test",
			eventsourcingdb.ReadEventsOptions{
				Recursive: false,
				LowerBound: &eventsourcingdb.Bound{
					ID:   "1",
					Type: eventsourcingdb.BoundTypeInclusive,
				},
			},
		) {
			assert.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		assert.Len(t, eventsRead, 1)

		var firstData EventData
		err = json.Unmarshal(eventsRead[0].Data, &firstData)
		require.NoError(t, err)
		assert.Equal(t, 42, firstData.Value)
	})

	t.Run("reads with upper bound", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		require.NoError(t, err)

		eventsRead := []eventsourcingdb.Event{}

		for event, err := range client.ReadEvents(
			ctx,
			"/test",
			eventsourcingdb.ReadEventsOptions{
				Recursive: false,
				UpperBound: &eventsourcingdb.Bound{
					ID:   "0",
					Type: eventsourcingdb.BoundTypeInclusive,
				},
			},
		) {
			assert.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		assert.Len(t, eventsRead, 1)

		var firstData EventData
		err = json.Unmarshal(eventsRead[0].Data, &firstData)
		require.NoError(t, err)
		assert.Equal(t, 23, firstData.Value)
	})

	t.Run("reads from latest event", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		firstEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test.foo",
			Data: EventData{
				Value: 23,
			},
		}

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test",
			Type:    "io.eventsourcingdb.test.bar",
			Data: EventData{
				Value: 42,
			},
		}

		_, err = client.WriteEvents(
			[]eventsourcingdb.EventCandidate{
				firstEvent,
				secondEvent,
			},
			nil,
		)
		require.NoError(t, err)

		eventsRead := []eventsourcingdb.Event{}

		for event, err := range client.ReadEvents(
			ctx,
			"/test",
			eventsourcingdb.ReadEventsOptions{
				Recursive: false,
				FromLatestEvent: &eventsourcingdb.ReadFromLatestEvent{
					Subject:          "/test",
					Type:             "io.eventsourcingdb.test.bar",
					IfEventIsMissing: eventsourcingdb.ReadEverythingIfEventIsMissing,
				},
			},
		) {
			assert.NoError(t, err)
			eventsRead = append(eventsRead, event)
		}

		assert.Len(t, eventsRead, 1)

		var firstData EventData
		err = json.Unmarshal(eventsRead[0].Data, &firstData)
		require.NoError(t, err)
		assert.Equal(t, 42, firstData.Value)
	})
}
