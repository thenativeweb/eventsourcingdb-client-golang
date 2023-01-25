package eventsourcingdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObserveEvents(t *testing.T) {
	janeRegistered := event.NewCandidate(events.TestSource, "/users/registered", events.Events.Registered.JaneDoe.Type, events.Events.Registered.JaneDoe.Data)
	johnRegistered := event.NewCandidate(events.TestSource, "/users/registered", events.Events.Registered.JohnDoe.Type, events.Events.Registered.JohnDoe.Data)
	janeLoggedIn := event.NewCandidate(events.TestSource, "/users/loggedIn", events.Events.LoggedIn.JaneDoe.Type, events.Events.LoggedIn.JaneDoe.Data)
	johnLoggedIn := event.NewCandidate(events.TestSource, "/users/loggedIn", events.Events.LoggedIn.JohnDoe.Type, events.Events.LoggedIn.JohnDoe.Data)

	prepareClientWithEvents := func(t *testing.T) eventsourcingdb.Client {
		client := database.WithoutAuthorization.GetClient()

		_, err := client.WriteEvents([]event.Candidate{
			janeRegistered,
			janeLoggedIn,
			johnRegistered,
			johnLoggedIn,
		})

		assert.NoError(t, err)

		return client
	}

	getNextEvent := func(t *testing.T, resultChan <-chan eventsourcingdb.ObserveEventsResult) event.Event {
		firstStoreItem := <-resultChan
		data, err := firstStoreItem.GetData()

		assert.NoError(t, err)

		return data.Event
	}

	matchRegisteredEvent := func(t *testing.T, event event.Event, expected events.RegisteredEvent) {
		assert.Equal(t, "/users/registered", event.Subject)
		assert.Equal(t, expected.Type, event.Type)

		var eventData events.RegisteredEventData
		err := json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	matchLoggedInEvent := func(t *testing.T, event event.Event, expected events.LoggedInEvent) {
		assert.Equal(t, "/users/loggedIn", event.Subject)
		assert.Equal(t, expected.Type, event.Type)

		var eventData events.LoggedInEventData
		err := json.Unmarshal(event.Data, &eventData)

		assert.NoError(t, err)

		assert.Equal(t, expected.Data.Name, eventData.Name)
	}

	t.Run("returns an error when trying to observe from a non-reachable server.", func(t *testing.T) {
		client := database.WithInvalidURL.GetClient()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(ctx, "/", eventsourcingdb.ObserveNonRecursively())

		firstResult := <-resultChan

		_, err := firstResult.GetData()
		assert.Error(t, err)
	})

	t.Run("supports authorization.", func(t *testing.T) {
		client := database.WithAuthorization.GetClient()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(ctx, "/", eventsourcingdb.ObserveNonRecursively())

		data, ok := <-resultChan

		assert.False(t, ok, fmt.Sprintf("unexpected data on result channel: %+v", data))
	})

	t.Run("observes events from a single subject.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(ctx, "/users/registered", eventsourcingdb.ObserveNonRecursively())

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, secondEvent, events.Events.Registered.JohnDoe)

		apfelFredCandidate := event.NewCandidate(
			events.TestSource,
			"/users/registered",
			events.Events.Registered.ApfelFred.Type,
			events.Events.Registered.ApfelFred.Data,
		)
		_, err := client.WriteEvents([]event.Candidate{
			apfelFredCandidate,
		})

		assert.NoError(t, err)
		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, events.Events.Registered.ApfelFred)
	})

	t.Run("observes events from a subject including child subjects.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JaneDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JaneDoe)

		thirdEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, thirdEvent, events.Events.Registered.JohnDoe)

		fourthEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, fourthEvent, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("observes events starting from the newest event matching the given event name.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(
			ctx,
			"/users/loggedIn",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLatestEvent(
				"/users/loggedIn",
				events.PrefixEventType("loggedin"),
				eventsourcingdb.ObserveNothing,
			),
		)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("observes events starting from the lower bound ID.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		resultChan := client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("2"),
		)

		firstEvent := getNextEvent(t, resultChan)
		matchRegisteredEvent(t, firstEvent, events.Events.Registered.JohnDoe)

		secondEvent := getNextEvent(t, resultChan)
		matchLoggedInEvent(t, secondEvent, events.Events.LoggedIn.JohnDoe)
	})

	t.Run("returns a ContextCanceledError when the context is canceled.", func(t *testing.T) {
		client := prepareClientWithEvents(t)
		ctx, cancel := context.WithCancel(context.Background())

		cancel()

		resultChan := client.ObserveEvents(
			ctx,
			"/users",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("2"),
		)

		_, err := (<-resultChan).GetData()
		assert.Error(t, err)
		assert.True(t, errors.IsContextCanceledError(err), fmt.Sprintf("%v", err))
	})

	t.Run("returns an error if mutually exclusive options are used", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		results := client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLowerBoundID("0"),
			eventsourcingdb.ObserveFromLatestEvent("/", "com.foo.bar", eventsourcingdb.WaitForEvent),
		)

		result := <-results
		_, err := result.GetData()

		assert.ErrorContains(t, err, "mutually exclusive")
	})

	t.Run("returns an error if incorrect options are used", func(t *testing.T) {
		client := database.WithoutAuthorization.GetClient()

		results := client.ObserveEvents(
			context.Background(),
			"/",
			eventsourcingdb.ObserveRecursively(),
			eventsourcingdb.ObserveFromLatestEvent("", "com.foo.bar", eventsourcingdb.WaitForEvent),
		)

		result := <-results
		_, err := result.GetData()

		assert.ErrorContains(t, err, "malformed event subject")
	})

	t.Run()
}
