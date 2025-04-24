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

func TestRunEventQLQuery(t *testing.T) {
	type EventData struct {
		Value int `json:"value"`
	}

	t.Run("reads no rows if the query does not return any rows", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdb.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		client, err := container.GetClient(ctx)
		require.NoError(t, err)

		didReadRows := false

		for range client.RunEventQLQuery(
			ctx,
			"FROM e IN events PROJECT INTO e",
		) {
			didReadRows = true
		}

		assert.False(t, didReadRows)
	})

	t.Run("reads all rows the query returns", func(t *testing.T) {
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

		rowsRead := []json.RawMessage{}

		for row, err := range client.RunEventQLQuery(
			ctx,
			"FROM e IN events PROJECT INTO e",
		) {
			require.NoError(t, err)
			rowsRead = append(rowsRead, row)
		}

		assert.Len(t, rowsRead, 2)

		var firstRow eventsourcingdb.Event
		err = json.Unmarshal(rowsRead[0], &firstRow)
		require.NoError(t, err)

		var firstData EventData
		err = json.Unmarshal(firstRow.Data, &firstData)
		require.NoError(t, err)

		assert.Equal(t, "0", firstRow.ID)
		assert.Equal(t, 23, firstData.Value)

		var secondRow eventsourcingdb.Event
		err = json.Unmarshal(rowsRead[1], &secondRow)
		require.NoError(t, err)

		var secondData EventData
		err = json.Unmarshal(secondRow.Data, &secondData)
		require.NoError(t, err)

		assert.Equal(t, "1", secondRow.ID)
		assert.Equal(t, 42, secondData.Value)
	})
}
