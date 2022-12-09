package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/test"
	"testing"
)

func TestReadEvents(t *testing.T) {
	client := database.WithoutAuthorization.GetClient()

	janeRegistered := eventsourcingdb.NewEventCandidate("/users/registered", test.Events.Registered.JaneDoe.Name, test.Events.Registered.JaneDoe.Data)
	johnRegistered := eventsourcingdb.NewEventCandidate("/users/registered", test.Events.Registered.JohnDoe.Name, test.Events.Registered.JohnDoe.Data)
	janeLoggedIn := eventsourcingdb.NewEventCandidate("/users/loggedIn", test.Events.LoggedIn.JaneDoe.Name, test.Events.LoggedIn.JaneDoe.Data)
	johnLoggedIn := eventsourcingdb.NewEventCandidate("/users/loggedIn", test.Events.LoggedIn.JohnDoe.Name, test.Events.LoggedIn.JohnDoe.Data)

	_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
		janeRegistered,
		janeLoggedIn,
		johnRegistered,
		johnLoggedIn,
	})

	assert.NoError(t, err)

	getNextEvent := func(t *testing.T, resultChan <-chan eventsourcingdb.ReadEventsResult) eventsourcingdb.Event {
		firstStoreItem := <-resultChan
		data, err := firstStoreItem.GetData()

		assert.NoError(t, err)

		return data.Event
	}

	matchRegisteredEvent := func(t *testing.T, event eventsourcingdb.Event, expected test.RegisteredEvent) {
		assert.Equal(t, "/users/registered", event.Metadata.StreamName)
		assert.Equal(t, expected.Name, event.Metadata.Name)

		var eventData test.RegisteredEventData
		err = json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	matchLoggedInEvent := func(t *testing.T, event eventsourcingdb.Event, expected test.LoggedInEvent) {
		assert.Equal(t, "/users/loggedIn", event.Metadata.StreamName)
		assert.Equal(t, expected.Name, event.Metadata.Name)

		var eventData test.LoggedInEventData
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
		matchRegisteredEvent(t, firstEvent, test.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, test.Events.Registered.JohnDoe)

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
		matchRegisteredEvent(t, firstEvent, test.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, test.Events.LoggedIn.JaneDoe)

		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, test.Events.Registered.JohnDoe)

		fourthEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, fourthEvent, test.Events.LoggedIn.JohnDoe)

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
		matchRegisteredEvent(t, firstEvent, test.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, test.Events.Registered.JaneDoe)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("reads events starting from the latest event matching the given event name.", func(t *testing.T) {
		resultChan := client.ReadEventsWithOptions(
			context.Background(),
			"/users/loggedIn",
			eventsourcingdb.NewReadEventsOptions(true).
				FromLatestEvent(eventsourcingdb.ReadFromLatestEvent{
					StreamName:       "/users/loggedIn",
					EventName:        "loggedIn",
					IfEventIsMissing: eventsourcingdb.ReadEverythingIfEventIsMissingDuringRead,
				}),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, firstEvent, test.Events.LoggedIn.JohnDoe)

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
		matchRegisteredEvent(t, firstEvent, test.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, test.Events.LoggedIn.JohnDoe)

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
		matchRegisteredEvent(t, firstEvent, test.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, test.Events.LoggedIn.JaneDoe)

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
