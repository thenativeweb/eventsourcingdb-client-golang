package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/test"
)

func TestObserveEvents(t *testing.T) {
	janeRegistered := eventsourcingdb.NewEventCandidate("/users/registered", test.Events.Registered.JaneDoe.Name, test.Events.Registered.JaneDoe.Data)
	johnRegistered := eventsourcingdb.NewEventCandidate("/users/registered", test.Events.Registered.JohnDoe.Name, test.Events.Registered.JohnDoe.Data)
	janeLoggedIn := eventsourcingdb.NewEventCandidate("/users/loggedIn", test.Events.LoggedIn.JaneDoe.Name, test.Events.LoggedIn.JaneDoe.Data)
	johnLoggedIn := eventsourcingdb.NewEventCandidate("/users/loggedIn", test.Events.LoggedIn.JohnDoe.Name, test.Events.LoggedIn.JohnDoe.Data)

	prepareClientWithEvents := func(t *testing.T) eventsourcingdb.Client {
		client := database.WithoutAuthorization.GetClient()

		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			janeRegistered,
			janeLoggedIn,
			johnRegistered,
			johnLoggedIn,
		})

		assert.NoError(t, err)

		return client
	}

	getNextEvent := func(t *testing.T, resultChan <-chan eventsourcingdb.ObserveEventsResult) eventsourcingdb.Event {
		firstStoreItem := <-resultChan
		data, err := firstStoreItem.GetData()

		assert.NoError(t, err)

		return data.Event
	}

	matchRegisteredEvent := func(t *testing.T, event eventsourcingdb.Event, expected test.RegisteredEvent) {
		assert.Equal(t, "/users/registered", event.Metadata.StreamName)
		assert.Equal(t, expected.Name, event.Metadata.Name)

		var eventData test.RegisteredEventData
		err := json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	matchLoggedInEvent := func(t *testing.T, event eventsourcingdb.Event, expected test.LoggedInEvent) {
		assert.Equal(t, "/users/loggedIn", event.Metadata.StreamName)
		assert.Equal(t, expected.Name, event.Metadata.Name)

		var eventData test.LoggedInEventData
		err := json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	t.Run("returns an error when trying to observe from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(ctx, "/", false)

		firstResult := <-resultChan

		_, err := firstResult.GetData()
		assert.Error(t, err)
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(ctx, "/", false)

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("observes from a single stream.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(ctx, "/users/registered", false)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, test.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, test.Events.Registered.JohnDoe)

		apfelFredCandidate := eventsourcingdb.NewEventCandidate(
			"/users/registered",
			test.Events.Registered.ApfelFred.Name,
			test.Events.Registered.ApfelFred.Data,
		)
		_, err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			apfelFredCandidate,
		})

		assert.NoError(t, err)
		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, test.Events.Registered.ApfelFred)
	})

	t.Run("observes from a stream including sub-streams.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEventsWithOptions(
			ctx,
			"/users",
			eventsourcingdb.NewObserveEventsOptions(true),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, test.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, test.Events.LoggedIn.JaneDoe)

		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, test.Events.Registered.JohnDoe)

		fourthEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, fourthEvent, test.Events.LoggedIn.JohnDoe)
	})

	t.Run("observes events starting from the newest event matching the given event name.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEventsWithOptions(
			ctx,
			"/users/loggedIn",
			eventsourcingdb.NewObserveEventsOptions(true).
				FromLatestEvent(eventsourcingdb.ObserveFromLatestEvent{
					StreamName:       "/users/loggedIn",
					EventName:        "loggedin",
					IfEventIsMissing: eventsourcingdb.ReadNothingIfEventIsMissingDuringObserve,
				}),
		)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, test.Events.LoggedIn.JohnDoe)
	})

	t.Run("observes events starting from the lower bound ID.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEventsWithOptions(
			ctx,
			"/users",
			eventsourcingdb.NewObserveEventsOptions(true).
				LowerBoundID(2),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, test.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, test.Events.LoggedIn.JohnDoe)
	})

	t.Run("returns a ContextCanceledError when the context is canceled.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())

		cancel()

		resultChan := client.ObserveEventsWithOptions(
			ctx,
			"/users",
			eventsourcingdb.NewObserveEventsOptions(true).
				LowerBoundID(2),
		)

		_, err := (<-resultChan).GetData()
		assert.Error(t, err)
		assert.True(t, errors.IsContextCanceledError(err), fmt.Sprintf("%v", err))
	})
}
