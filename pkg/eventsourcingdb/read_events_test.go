package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"testing"
)

func TestReadEvents(t *testing.T) {
	client := database.WithoutAuthorization.GetClient()

	janeRegistered := event.NewCandidate(events.TestSource, "/users/registered", events.Events.Registered.JaneDoe.Type, events.Events.Registered.JaneDoe.Data)
	johnRegistered := event.NewCandidate(events.TestSource, "/users/registered", events.Events.Registered.JohnDoe.Type, events.Events.Registered.JohnDoe.Data)
	janeLoggedIn := event.NewCandidate(events.TestSource, "/users/loggedIn", events.Events.LoggedIn.JaneDoe.Type, events.Events.LoggedIn.JaneDoe.Data)
	johnLoggedIn := event.NewCandidate(events.TestSource, "/users/loggedIn", events.Events.LoggedIn.JohnDoe.Type, events.Events.LoggedIn.JohnDoe.Data)

	_, err := client.WriteEvents([]event.Candidate{
		janeRegistered,
		janeLoggedIn,
		johnRegistered,
		johnLoggedIn,
	})

	assert.NoError(t, err)

	getNextEvent := func(t *testing.T, resultChan <-chan eventsourcingdb.ReadEventsResult) event.Event {
		firstStoreItem := <-resultChan
		data, err := firstStoreItem.GetData()

		assert.NoError(t, err)

		return data.Event
	}

	matchRegisteredEvent := func(t *testing.T, event event.Event, expected events.RegisteredEvent) {
		assert.Equal(t, "/users/registered", event.Subject)
		assert.Equal(t, expected.Type, event.Type)

		var eventData events.RegisteredEventData
		err = json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	matchLoggedInEvent := func(t *testing.T, event event.Event, expected events.LoggedInEvent) {
		assert.Equal(t, "/users/loggedIn", event.Subject)
		assert.Equal(t, expected.Type, event.Type)

		var eventData events.LoggedInEventData
		err = json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	t.Run("returns an error when trying to read from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()

		resultChan := client.ReadEvents(context.Background(), "/", false)

		firstResult := <-resultChan

		_, err := firstResult.GetData()
		assert.Error(t, err)
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()

		resultChan := client.ReadEvents(context.Background(), "/", false)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads from a single stream.", func(t *testing.T) {
		resultChan := client.ReadEvents(context.Background(), "/users/registered", false)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, events.Events.Registered.JohnDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads from a stream including sub-streams.", func(t *testing.T) {
		resultChan := client.ReadEventsWithOptions(
			context.Background(),
			"/users",
			eventsourcingdb.NewReadEventsOptions(true),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JaneDoe)

		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, events.Events.Registered.JohnDoe)

		fourthEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, fourthEvent, events.Events.LoggedIn.JohnDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads the events in non-chronological order.", func(t *testing.T) {
		resultChan := client.ReadEventsWithOptions(
			context.Background(),
			"/users/registered",
			eventsourcingdb.NewReadEventsOptions(false).
				Chronological(false),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, events.Events.Registered.JaneDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads events starting from the latest event matching the given event name.", func(t *testing.T) {
		resultChan := client.ReadEventsWithOptions(
			context.Background(),
			"/users/loggedIn",
			eventsourcingdb.NewReadEventsOptions(true).
				FromLatestEvent(eventsourcingdb.ReadFromLatestEvent{
					Subject:          "/users/loggedIn",
					Type:             events.PrefixEventType("loggedIn"),
					IfEventIsMissing: eventsourcingdb.ReadEverythingIfEventIsMissingDuringRead,
				}),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, firstEvent, events.Events.LoggedIn.JohnDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads events starting from the lower bound ID.", func(t *testing.T) {
		resultChan := client.ReadEventsWithOptions(
			context.Background(),
			"/users",
			eventsourcingdb.NewReadEventsOptions(true).
				LowerBoundID(2),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JohnDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads events up to the upper bound ID.", func(t *testing.T) {
		resultChan := client.ReadEventsWithOptions(
			context.Background(),
			"/users",
			eventsourcingdb.NewReadEventsOptions(true).
				UpperBoundID(1),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JaneDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("returns a ContextCanceledError when the context is canceled.", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		resultChan := client.ReadEventsWithOptions(
			ctx,
			"/users",
			eventsourcingdb.NewReadEventsOptions(true).
				UpperBoundID(1),
		)

		_, err := (<-resultChan).GetData()
		assert.Error(t, err)
		assert.True(t, errors.IsContextCanceledError(err))
	})
}
