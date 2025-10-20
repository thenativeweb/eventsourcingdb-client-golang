package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestWriteEvents(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("writes a single event", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
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
			[]eventsourcingdb.EventCandidate{
				event,
			},
			nil,
		)
		assert.NoError(t, err)
		assert.Len(t, writtenEvents, 1)
		assert.Equal(t, "0", writtenEvents[0].ID)
	})

	t.Run("writes multiple events", func(t *testing.T) {
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

		writtenEvents, err := client.WriteEvents(
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

		_, err = client.WriteEvents(
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
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectPristinePrecondition("/test"),
			},
		)

		assert.EqualError(t, err, "failed to write events, got HTTP status code '409', expected '200'")
	})

	t.Run("supports the isSubjectPopulated precondition", func(t *testing.T) {
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
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test"),
			},
		)
		assert.EqualError(t, err, "failed to write events, got HTTP status code '409', expected '200'")

		_, err = client.WriteEvents(
			[]eventsourcingdb.EventCandidate{
				firstEvent,
			},
			nil,
		)
		require.NoError(t, err)

		writtenEvents, err := client.WriteEvents(
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

		_, err = client.WriteEvents(
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
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "1"),
			},
		)

		assert.EqualError(t, err, "failed to write events, got HTTP status code '409', expected '200'")
	})

	t.Run("supports the isEventQlQueryTrue precondition", func(t *testing.T) {
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

		_, err = client.WriteEvents(
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
			[]eventsourcingdb.EventCandidate{
				secondEvent,
			},
			[]eventsourcingdb.Precondition{
				eventsourcingdb.NewIsEventQLQueryTruePrecondition("FROM e IN events PROJECT INTO COUNT() == 0"),
			},
		)

		assert.EqualError(t, err, "failed to write events, got HTTP status code '409', expected '200'")
	})

}
