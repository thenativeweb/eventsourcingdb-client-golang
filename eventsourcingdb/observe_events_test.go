package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
)

func TestObserveEvents(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("observes no events if the database is empty", func(t *testing.T) {
		ctx := context.Background()

		container := eventsourcingdb.NewContainer()
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		didObserveEvents := false
		innerCtx, cancel := context.WithCancel(ctx)

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		for _, err := range client.ObserveEvents(
			innerCtx,
			"/",
			eventsourcingdb.ObserveEventsOptions{
				Recursive: true,
			},
		) {
			assert.NoError(t, err)
			didObserveEvents = true
		}

		assert.False(t, didObserveEvents)
	})

	t.Run("observes all events from the given subject", func(t *testing.T) {
		ctx := context.Background()

		container := eventsourcingdb.NewContainer()
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

		eventsObserved := []eventsourcingdb.Event{}
		innerCtx, cancel := context.WithCancel(ctx)

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		for event, err := range client.ObserveEvents(
			innerCtx,
			"/",
			eventsourcingdb.ObserveEventsOptions{
				Recursive: true,
			},
		) {
			assert.NoError(t, err)
			eventsObserved = append(eventsObserved, event)
		}

		assert.Len(t, eventsObserved, 2)
	})

	t.Run("observes with lower bound", func(t *testing.T) {
		ctx := context.Background()

		container := eventsourcingdb.NewContainer()
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

		eventsObserved := []eventsourcingdb.Event{}
		innerCtx, cancel := context.WithCancel(ctx)

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		for event, err := range client.ObserveEvents(
			innerCtx,
			"/",
			eventsourcingdb.ObserveEventsOptions{
				Recursive: true,
				LowerBound: &eventsourcingdb.Bound{
					ID:   "1",
					Type: eventsourcingdb.BoundTypeInclusive,
				},
			},
		) {
			assert.NoError(t, err)
			eventsObserved = append(eventsObserved, event)
		}

		assert.Len(t, eventsObserved, 1)

		var firstData EventData
		err = json.Unmarshal(eventsObserved[0].Data, &firstData)
		require.NoError(t, err)
		assert.Equal(t, 42, firstData.Value)
	})

	t.Run("observes from latest event", func(t *testing.T) {
		ctx := context.Background()

		container := eventsourcingdb.NewContainer()
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

		eventsObserved := []eventsourcingdb.Event{}
		innerCtx, cancel := context.WithCancel(ctx)

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		for event, err := range client.ObserveEvents(
			innerCtx,
			"/",
			eventsourcingdb.ObserveEventsOptions{
				Recursive: true,
				FromLatestEvent: &eventsourcingdb.ObserveFromLatestEvent{
					Subject:          "/test",
					Type:             "io.eventsourcingdb.test.bar",
					IfEventIsMissing: eventsourcingdb.ObserveEverythingIfEventIsMissing,
				},
			},
		) {
			assert.NoError(t, err)
			eventsObserved = append(eventsObserved, event)
		}

		assert.Len(t, eventsObserved, 1)

		var firstData EventData
		err = json.Unmarshal(eventsObserved[0].Data, &firstData)
		require.NoError(t, err)
		assert.Equal(t, 42, firstData.Value)
	})
}
