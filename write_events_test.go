package eventsourcingdb_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/test"
)

func TestWriteEvents(t *testing.T) {
	t.Run("returns an error when trying to write to a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.Error(t, err)
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.NoError(t, err)
	})

	t.Run("writes a single event.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.NoError(t, err)
	})

	t.Run("writes multiple events.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe
		johnRegistered := test.Events.Registered.JohnDoe

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
			eventsourcingdb.NewEventCandidate(streamName, johnRegistered.Name, johnRegistered.Data),
		})

		assert.NoError(t, err)
	})

	t.Run("returns an error when trying to write an empty list of events.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()
		err := client.WriteEvents([]eventsourcingdb.EventCandidate{})

		assert.Error(t, err)
	})
}

func TestWriteEventsWithPreconditions(t *testing.T) {
	t.Run("when using the 'is stream pristine' precondition", func(t *testing.T) {
		t.Run("writes events if the stream is pristine.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()

			streamName := "/" + uuid.New().String()
			janeRegistered := test.Events.Registered.JaneDoe

			err := client.WriteEventsWithPreconditions([]interface{}{
				eventsourcingdb.NewIsStreamPristinePrecondition(streamName),
			}, []eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
			})

			assert.NoError(t, err)
		})

		t.Run("returns an error if the stream is not pristine.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()

			streamName := "/" + uuid.New().String()
			janeRegistered := test.Events.Registered.JaneDoe
			johnRegistered := test.Events.Registered.JohnDoe

			err := client.WriteEvents([]eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
			})

			assert.NoError(t, err)

			err = client.WriteEventsWithPreconditions([]interface{}{
				eventsourcingdb.NewIsStreamPristinePrecondition(streamName),
			}, []eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate(streamName, johnRegistered.Name, johnRegistered.Data),
			})

			assert.Error(t, err)
		})
	})

	t.Run("when using the 'is stream on event ID' precondition", func(t *testing.T) {
		t.Run("writes events if the last event in the stream has the given event ID.", func(t *testing.T) {
			// TODO: We can't read the last eventID yet, so we can't implement
			//       the test yet. This test should not be done by reading the
			//			 event ID, but by extending the server to return the written
			//       ID (see https://github.com/thenativeweb/eventsourcingdb/issues/188).
		})

		t.Run("returns an error if the last event in the stream does not have the given event ID.", func(t *testing.T) {
			// TODO: We can't read the last eventID yet, so we can't implement
			//       the test yet. This test should not be done by reading the
			//			 event ID, but by extending the server to return the written
			//       ID (see https://github.com/thenativeweb/eventsourcingdb/issues/188).
		})
	})
}
