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

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.Error(t, err)
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.NoError(t, err)
	})

	t.Run("writes a single event.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
		})

		assert.NoError(t, err)
	})

	t.Run("returns the written event metadata.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		janeRegistered := test.Events.Registered.JaneDoe
		johnRegistered := test.Events.Registered.JohnDoe
		johnLoggedIn := test.Events.LoggedIn.JohnDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate("/users/registered", janeRegistered.Name, janeRegistered.Data),
		})
		assert.NoError(t, err)

		writtenEventsMetadata, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate("/users/registered", johnRegistered.Name, johnRegistered.Data),
			eventsourcingdb.NewEventCandidate("/users/loggedIn", johnLoggedIn.Name, johnLoggedIn.Data),
		})

		assert.Len(t, writtenEventsMetadata, 2)
		assert.Equal(t, writtenEventsMetadata[0].Name, "registered")
		assert.Equal(t, writtenEventsMetadata[0].StreamName, "/users/registered")
		assert.Equal(t, writtenEventsMetadata[0].ID, 1)
		assert.Equal(t, writtenEventsMetadata[1].Name, "loggedIn")
		assert.Equal(t, writtenEventsMetadata[1].StreamName, "/users/loggedIn")
		assert.Equal(t, writtenEventsMetadata[1].ID, 2)

		assert.NoError(t, err)
	})

	t.Run("writes multiple events.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		streamName := "/" + uuid.New().String()
		janeRegistered := test.Events.Registered.JaneDoe
		johnRegistered := test.Events.Registered.JohnDoe

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
			eventsourcingdb.NewEventCandidate(streamName, johnRegistered.Name, johnRegistered.Data),
		})

		assert.NoError(t, err)
	})

	t.Run("returns an error when trying to write an empty list of events.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()
		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{})

		assert.Error(t, err)
	})
}

func TestWriteEventsWithPreconditions(t *testing.T) {
	t.Run("when using the 'is stream pristine' precondition", func(t *testing.T) {
		t.Run("writes events if the stream is pristine.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()

			streamName := "/" + uuid.New().String()
			janeRegistered := test.Events.Registered.JaneDoe

			_, err := client.WriteEventsWithPreconditions(
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

			_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate(streamName, janeRegistered.Name, janeRegistered.Data),
			})

			assert.NoError(t, err)

			_, err = client.WriteEventsWithPreconditions(
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

			_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate("/users", janeRegistered.Name, janeRegistered.Data),
				eventsourcingdb.NewEventCandidate("/users", johnRegistered.Name, johnRegistered.Data),
			})

			assert.NoError(t, err)

			events := client.ReadEvents(context.Background(), "/users", false)

			var lastEventID int
			for event := range events {
				data, err := event.GetData()
				assert.NoError(t, err)

				lastEventID = data.Event.Metadata.ID
			}

			_, err = client.WriteEventsWithPreconditions(
				eventsourcingdb.NewPreconditions().IsStreamOnEventID("/users", lastEventID),
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

			_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate("/users", janeRegistered.Name, janeRegistered.Data),
				eventsourcingdb.NewEventCandidate("/users", johnRegistered.Name, johnRegistered.Data),
			})

			assert.NoError(t, err)

			lastEventID := 1337

			_, err = client.WriteEventsWithPreconditions(
				eventsourcingdb.NewPreconditions().IsStreamOnEventID("/users", lastEventID),
				[]eventsourcingdb.EventCandidate{
					eventsourcingdb.NewEventCandidate("/users", fredRegistered.Name, fredRegistered.Data),
				},
			)

			assert.Error(t, err)
		})
	})
}
