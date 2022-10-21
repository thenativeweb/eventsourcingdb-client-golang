package eventsourcingdb_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/test"
	"testing"
)

func TestReadStreamNames(t *testing.T) {
	t.Run("returns a channel containing an error when trying to read from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.Client

		readStreamNameResults := client.ReadStreamNames(context.Background())

		errorResult := <-readStreamNameResults
		assert.True(t, errorResult.IsError())
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.Client

		readStreamNameResults := client.ReadStreamNames(context.Background())

		for result := range readStreamNameResults {
			_, err := result.GetData()
			assert.NoError(t, err)
		}
	})

	t.Run("closes the channel when no more stream names exist.", func(t *testing.T) {
		client := database.WithoutAuthorization.Client

		readStreamNameResults := client.ReadStreamNames(context.Background())

		rootStreamNameResult := <-readStreamNameResults
		rootStreamName, err := rootStreamNameResult.GetData()

		assert.NoError(t, err)
		assert.Equal(t, rootStreamName, "/")

		_, ok := <-readStreamNameResults
		assert.False(t, ok)
	})

	t.Run("reads all stream names starting from /.", func(t *testing.T) {
		client := database.WithoutAuthorization.Client

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.NoError(t, err)

		readStreamNameResults := client.ReadStreamNames(context.Background())
		streamNames := make([]string, 0, 2)

		for result := range readStreamNameResults {
			data, err := result.GetData()
			assert.NoError(t, err)

			streamNames = append(streamNames, data)
		}

		assert.Equal(t, streamNames, []string{"/", streamName})
	})

	t.Run("reads stream names starting from the given base stream name.", func(t *testing.T) {
		client := database.WithoutAuthorization.Client

		streamName := "/foobar/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.NoError(t, err)

		readStreamNameResults := client.ReadStreamNamesWithBaseStreamName(context.Background(), "/foobar")
		streamNames := make([]string, 0, 2)

		for result := range readStreamNameResults {
			data, err := result.GetData()
			assert.NoError(t, err)

			streamNames = append(streamNames, data)
		}

		assert.Equal(t, streamNames, []string{"/foobar", streamName})
	})

	t.Run("closes the result channel when the given context is cancelled.", func(t *testing.T) {
		client := database.WithoutAuthorization.Client

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		readStreamNameResults := client.ReadStreamNames(ctx)

		cancelledResult := <-readStreamNameResults
		_, err := cancelledResult.GetData()
		assert.Error(t, err, errors.New("context cancelled"))

		superfluousResult, ok := <-readStreamNameResults
		assert.False(t, ok, fmt.Sprintf("channel did not close %+v", superfluousResult))
	})
}
