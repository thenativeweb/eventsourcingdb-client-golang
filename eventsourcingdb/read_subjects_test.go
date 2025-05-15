package eventsourcingdb_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestReadSubjects(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("reads no subjects if the database is empty", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		didReadSubjects := false

		for _, err := range client.ReadSubjects(
			ctx,
			"/",
		) {
			assert.NoError(t, err)
			didReadSubjects = true
		}

		assert.False(t, didReadSubjects)
	})

	t.Run("reads all subjects", func(t *testing.T) {
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
			Subject: "/test/1",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test/2",
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

		subjectsRead := []string{}

		for subject, err := range client.ReadSubjects(
			ctx,
			"/",
		) {
			assert.NoError(t, err)
			subjectsRead = append(subjectsRead, subject)
		}

		assert.Len(t, subjectsRead, 4)
		assert.Equal(t, subjectsRead[0], "/")
		assert.Equal(t, subjectsRead[1], "/test")
		assert.Equal(t, subjectsRead[2], "/test/1")
		assert.Equal(t, subjectsRead[3], "/test/2")
	})

	t.Run("reads all subjects from the base subject", func(t *testing.T) {
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
			Subject: "/test/1",
			Type:    "io.eventsourcingdb.test",
			Data: EventData{
				Value: 23,
			},
		}

		secondEvent := eventsourcingdb.EventCandidate{
			Source:  "https://www.eventsourcingdb.io",
			Subject: "/test/2",
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

		subjectsRead := []string{}

		for subject, err := range client.ReadSubjects(
			ctx,
			"/test",
		) {
			assert.NoError(t, err)
			subjectsRead = append(subjectsRead, subject)
		}

		assert.Len(t, subjectsRead, 3)
		assert.Equal(t, subjectsRead[0], "/test")
		assert.Equal(t, subjectsRead[1], "/test/1")
		assert.Equal(t, subjectsRead[2], "/test/2")
	})
}
