package eventsourcingdb_test

import (
	"context"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWriteEvents(t *testing.T) {
	t.Run("returns an error when trying to write to a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()
		source := event.NewSource(events.TestSource)

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.Error(t, err)
	})

	t.Run("returns an error if a candidate subject is malformed", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		_, err := client.WriteEvents(
			[]event.Candidate{
				event.NewCandidate("tag:foobar.com,2023:barbaz", "foobar", "com.foobar.barbaz", struct{}{}),
			},
		)
		assert.ErrorContains(t, err, "malformed event subject")
	})

	t.Run("returns an error if a candidate type is malformed", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		_, err := client.WriteEvents(
			[]event.Candidate{
				event.NewCandidate("tag:foobar.com,2023:barbaz", "/foobar", "barbaz", struct{}{}),
			},
		)
		assert.ErrorContains(t, err, "malformed event type")
	})

	t.Run("returns an error if a candidate source is malformed", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		_, err := client.WriteEvents(
			[]event.Candidate{
				event.NewCandidate("://wurstsoße", "/foobar", "com.foobar.barbaz", struct{}{}),
			},
		)
		assert.ErrorContains(t, err, "malformed event source")
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.NoError(t, err)
	})

	t.Run("writes a single event.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			},
		)

		assert.NoError(t, err)
	})

	t.Run("returns the written event metadata.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		janeRegistered := events.Events.Registered.JaneDoe
		johnRegistered := events.Events.Registered.JohnDoe
		johnLoggedIn := events.Events.LoggedIn.JohnDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent("/users/registered", janeRegistered.Type, janeRegistered.Data),
			},
		)
		assert.NoError(t, err)

		writtenEventsMetadata, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent("/users/registered", johnRegistered.Type, johnRegistered.Data),
				source.NewEvent("/users/loggedIn", johnLoggedIn.Type, johnLoggedIn.Data),
			},
		)

		assert.Len(t, writtenEventsMetadata, 2)
		assert.Equal(t, events.TestSource, writtenEventsMetadata[0].Source)
		assert.Equal(t, events.PrefixEventType("registered"), writtenEventsMetadata[0].Type)
		assert.Equal(t, "/users/registered", writtenEventsMetadata[0].Subject)
		assert.Equal(t, "1", writtenEventsMetadata[0].ID)
		assert.Equal(t, events.TestSource, writtenEventsMetadata[1].Source)
		assert.Equal(t, events.PrefixEventType("loggedIn"), writtenEventsMetadata[1].Type)
		assert.Equal(t, "/users/loggedIn", writtenEventsMetadata[1].Subject)
		assert.Equal(t, "2", writtenEventsMetadata[1].ID)

		assert.NoError(t, err)
	})

	t.Run("writes multiple events.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()
		source := event.NewSource(events.TestSource)

		subject := "/" + uuid.New().String()
		janeRegistered := events.Events.Registered.JaneDoe
		johnRegistered := events.Events.Registered.JohnDoe

		_, err := client.WriteEvents(
			[]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
				source.NewEvent(subject, johnRegistered.Type, johnRegistered.Data),
			},
		)
		assert.NoError(t, err)
	})

	t.Run("returns an error when trying to write an empty list of events.", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()
		_, err := client.WriteEvents([]event.Candidate{})

		assert.Error(t, err)
	})
}

func TestWriteEventsWithPreconditions(t *testing.T) {
	t.Run("when using the 'is stream pristine' precondition", func(t *testing.T) {
		t.Run("writes events if the stream is pristine.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()
			source := event.NewSource(events.TestSource)

			subject := "/" + uuid.New().String()
			janeRegistered := events.Events.Registered.JaneDoe

			_, err := client.WriteEvents(
				[]event.Candidate{
					source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
				},
				eventsourcingdb.IsSubjectPristine(subject),
			)

			assert.NoError(t, err)
		})

		t.Run("returns an error if the stream is not pristine.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()
			source := event.NewSource(events.TestSource)

			subject := "/" + uuid.New().String()
			janeRegistered := events.Events.Registered.JaneDoe
			johnRegistered := events.Events.Registered.JohnDoe

			_, err := client.WriteEvents([]event.Candidate{
				source.NewEvent(subject, janeRegistered.Type, janeRegistered.Data),
			})

			assert.NoError(t, err)

			_, err = client.WriteEvents(
				[]event.Candidate{
					source.NewEvent(subject, johnRegistered.Type, johnRegistered.Data),
				},
				eventsourcingdb.IsSubjectPristine(subject),
			)

			assert.Error(t, err)
		})
	})

	t.Run("when using the 'is stream on event ID' precondition", func(t *testing.T) {
		t.Run("writes events if the last event in the stream has the given event ID.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()
			source := event.NewSource(events.TestSource)

			janeRegistered := events.Events.Registered.JaneDoe
			johnRegistered := events.Events.Registered.JohnDoe
			fredRegistered := events.Events.Registered.ApfelFred

			_, err := client.WriteEvents(
				[]event.Candidate{
					source.NewEvent("/users", janeRegistered.Type, janeRegistered.Data),
					source.NewEvent("/users", johnRegistered.Type, johnRegistered.Data),
				},
			)

			assert.NoError(t, err)

			readEvents := client.ReadEvents(context.Background(), "/users", eventsourcingdb.ReadNonRecursively())

			var lastEventID string
			for readEvent := range readEvents {
				data, err := readEvent.GetData()
				assert.NoError(t, err)

				lastEventID = data.Event.ID
			}

			_, err = client.WriteEvents(
				[]event.Candidate{
					source.NewEvent("/users", fredRegistered.Type, fredRegistered.Data),
				},
				eventsourcingdb.IsSubjectOnEventID("/users", lastEventID),
			)

			assert.NoError(t, err)
		})

		t.Run("returns an error if the last event in the stream does not have the given event ID.", func(t *testing.T) {
			client := database.WithoutAuthorization.GetClient()
			source := event.NewSource(events.TestSource)

			janeRegistered := events.Events.Registered.JaneDoe
			johnRegistered := events.Events.Registered.JohnDoe
			fredRegistered := events.Events.Registered.ApfelFred

			_, err := client.WriteEvents(
				[]event.Candidate{
					source.NewEvent("/users", janeRegistered.Type, janeRegistered.Data),
					source.NewEvent("/users", johnRegistered.Type, johnRegistered.Data),
				},
			)

			assert.NoError(t, err)

			lastEventID := "1337"

			_, err = client.WriteEvents(
				[]event.Candidate{
					source.NewEvent("/users", fredRegistered.Type, fredRegistered.Data),
				},
				eventsourcingdb.IsSubjectOnEventID("/users", lastEventID),
			)

			assert.Error(t, err)
		})
	})
}
