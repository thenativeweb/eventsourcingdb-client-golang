package eventsourcingdb_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/test"
	"testing"
)

func TestReadStreamNames(t *testing.T) {
	t.Run("returns a channel containing an error when trying to read from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()

		readStreamNameResults := client.ReadStreamNames(context.Background())

		errorResult := <-readStreamNameResults
		assert.True(t, errorResult.IsError())
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		readStreamNameResults := client.ReadStreamNames(context.Background())

		for result := range readStreamNameResults {
			_, err := result.GetData()
			assert.NoError(t, err)
		}
	})

	t.Run("closes the channel when no more stream names exist.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		readStreamNameResults := client.ReadStreamNames(context.Background())

		rootStreamNameResult := <-readStreamNameResults
		rootStreamName, err := rootStreamNameResult.GetData()

		assert.NoError(t, err)
		assert.Equal(t, eventsourcingdb.StreamName{"/"}, rootStreamName)

		_, ok := <-readStreamNameResults
		assert.False(t, ok)
	})

	t.Run("reads all stream names starting from /.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.NoError(t, err)

		readStreamNameResults := client.ReadStreamNames(context.Background())
		streamNames := make([]eventsourcingdb.StreamName, 0, 2)

		for result := range readStreamNameResults {
			data, err := result.GetData()
			assert.NoError(t, err)

			streamNames = append(streamNames, data)
		}

		assert.Equal(t, []eventsourcingdb.StreamName{{"/"}, {streamName}}, streamNames)
	})

	t.Run("reads stream names starting from the given base stream name.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		streamName := "/foobar/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.NoError(t, err)

		readStreamNameResults := client.ReadStreamNamesWithBaseStreamName(context.Background(), "/foobar")
		streamNames := make([]eventsourcingdb.StreamName, 0, 2)

		for result := range readStreamNameResults {
			data, err := result.GetData()
			assert.NoError(t, err)

			streamNames = append(streamNames, data)
		}

		assert.Equal(t, []eventsourcingdb.StreamName{{"/foobar"}, {streamName}}, streamNames)
	})

	t.Run("closes the result channel when the given context is canceled.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		readStreamNameResults := client.ReadStreamNames(ctx)

		cancelledResult := <-readStreamNameResults
		_, err := cancelledResult.GetData()
		assert.Error(t, err)
		assert.True(t, customErrors.IsContextCanceledError(err))

		superfluousResult, ok := <-readStreamNameResults
		assert.False(t, ok, fmt.Sprintf("channel did not close %+v", superfluousResult))
	})
}
