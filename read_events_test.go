package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/test"
	"testing"
)

func TestReadEvents(t *testing.T) {
	client := database.WithoutAuthorization.GetClient()

	janeRegistered := eventsourcingdb.NewEventCandidate("/users/registered", test.Events.Registered.JaneDoe.Name, test.Events.Registered.JaneDoe.Data)
	johnRegistered := eventsourcingdb.NewEventCandidate("/users/registered", test.Events.Registered.JohnDoe.Name, test.Events.Registered.JohnDoe.Data)
	janeLoggedIn := eventsourcingdb.NewEventCandidate("/users/loggedIn", test.Events.LoggedIn.JaneDoe.Name, test.Events.LoggedIn.JaneDoe.Data)
	johnLoggedIn := eventsourcingdb.NewEventCandidate("/users/loggedIn", test.Events.LoggedIn.JohnDoe.Name, test.Events.LoggedIn.JohnDoe.Data)

	err := client.WriteEvents([]eventsourcingdb.EventCandidate{
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

	matchRegisteredEvent := func(t *testing.T, event eventsourcingdb.Event, candidate eventsourcingdb.EventCandidate) {
		assert.Equal(t, event.Metadata.StreamName, candidate.Metadata.StreamName)
		assert.Equal(t, event.Metadata.Name, candidate.Metadata.Name)

		var eventData test.RegisteredEventData
		err = json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		candidateData, ok := candidate.Data.(test.RegisteredEventData)

		assert.True(t, ok)

		assert.Equal(t, candidateData.Name, eventData.Name)
	}

	matchLoggedInEvent := func(t *testing.T, event eventsourcingdb.Event, candidate eventsourcingdb.EventCandidate) {
		assert.Equal(t, event.Metadata.StreamName, candidate.Metadata.StreamName)
		assert.Equal(t, event.Metadata.Name, candidate.Metadata.Name)

		var eventData test.LoggedInEventData
		err = json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		candidateData, ok := candidate.Data.(test.LoggedInEventData)

		assert.True(t, ok)

		assert.Equal(t, candidateData.Name, eventData.Name)
	}

	t.Run("reads from a single stream.", func(t *testing.T) {
		resultChan := client.ReadEvents(context.Background(), "/users/registered", false)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, janeRegistered)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, johnRegistered)

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
		matchRegisteredEvent(t, firstEvent, janeRegistered)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, janeLoggedIn)

		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, johnRegistered)

		fourthEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, fourthEvent, johnLoggedIn)

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
		matchRegisteredEvent(t, firstEvent, johnRegistered)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, janeRegistered)

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
		matchLoggedInEvent(t, firstEvent, johnLoggedIn)

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
		matchRegisteredEvent(t, firstEvent, johnRegistered)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, johnLoggedIn)

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
		matchRegisteredEvent(t, firstEvent, janeRegistered)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, janeLoggedIn)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})
}
