package eventsourcingdb_test

import (
	"context"
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

			err := client.WriteEventsWithPreconditions(
				eventsourcingdb.NewPreconditions().IsStreamPristine(streamName),
				[]eventsourcingdb.EventCandidate{
					eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
				},
			)

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

			err = client.WriteEventsWithPreconditions(
				eventsourcingdb.NewPreconditions().IsStreamPristine(streamName),
				[]eventsourcingdb.EventCandidate{
					eventsourcingdb.NewEventCandidate(streamName, johnRegistered.Name, johnRegistered.Data),
				},
			)

			assert.Error(t, err)
		})
	})

	t.Run("when using the 'is stream on event ID' precondition", func(t *testing.T) {
		t.Run("writes events if the last event in the stream has the given event ID.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()

			janeRegistered := test.Events.Registered.JaneDoe
			johnRegistered := test.Events.Registered.JohnDoe
			fredRegistered := test.Events.Registered.ApfelFred

			err := client.WriteEvents([]eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate("/users", janeRegistered.Name, janeRegistered.Data),
				eventsourcingdb.NewEventCandidate("/users", johnRegistered.Name, johnRegistered.Data),
			})

			assert.NoError(t, err)

			events := client.ReadEvents(context.Background(), "/users", false)

			var lastEventId int
			for event := range events {
				data, err := event.GetData()
				assert.NoError(t, err)

				lastEventId = data.Event.Metadata.ID
			}

			err = client.WriteEventsWithPreconditions(
				eventsourcingdb.NewPreconditions().IsStreamOnEventID("/users", lastEventId),
				// TODO: think about the following edge case:
				// the precondition applies to some stream(s), but we can write to arbitrary streams, since the
				// stream name is part of the event candidate
				[]eventsourcingdb.EventCandidate{
					eventsourcingdb.NewEventCandidate("/users", fredRegistered.Name, fredRegistered.Data),
				},
			)

			assert.NoError(t, err)
		})

		t.Run("returns an error if the last event in the stream does not have the given event ID.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()

			janeRegistered := test.Events.Registered.JaneDoe
			johnRegistered := test.Events.Registered.JohnDoe
			fredRegistered := test.Events.Registered.ApfelFred

			err := client.WriteEvents([]eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate("/users", janeRegistered.Name, janeRegistered.Data),
				eventsourcingdb.NewEventCandidate("/users", johnRegistered.Name, johnRegistered.Data),
			})

			assert.NoError(t, err)

			lastEventId := 1337

			err = client.WriteEventsWithPreconditions(
				eventsourcingdb.NewPreconditions().IsStreamOnEventID("/users", lastEventId),
				[]eventsourcingdb.EventCandidate{
					eventsourcingdb.NewEventCandidate("/users", fredRegistered.Name, fredRegistered.Data),
				},
			)

			assert.Error(t, err)
		})
	})
}
