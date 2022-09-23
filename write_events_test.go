package eventsourcingdb_test

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"testing"
)

type testPayload struct {
	TestData string `json:"testData"`
}

func TestWriteEvents(t *testing.T) {
	t.Run("writes events to the database.", func(t *testing.T) {
		client := eventsourcingdb.NewClient(baseURLWithoutAuthorization)
		streamName := "/" + uuid.New().String()

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "1"}),
			eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "2"}),
			eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "3"}),
			eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "4"}),
		})

		assert.NoError(t, err)
	})

	t.Run("returns an error when trying to write events to a nonexistent database.", func(t *testing.T) {
		client := eventsourcingdb.NewClient("schwibbedi.invalid")
		streamName := "/" + uuid.New().String()

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "1"}),
			eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "2"}),
			eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "3"}),
			eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "4"}),
		})

		assert.Error(t, err)
	})

	t.Run("returns an error when trying to write an empty list of events.", func(t *testing.T) {
		client := eventsourcingdb.NewClient(baseURLWithoutAuthorization)

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{})

		assert.Error(t, err)
	})
}

func TestWriteEventsWithPreconditions(t *testing.T) {
	t.Run("when using the IsStreamPristinePrecondition", func(t *testing.T) {
		t.Run("writes events if the stream is pristine.", func(t *testing.T) {
			client := eventsourcingdb.NewClient(baseURLWithoutAuthorization)
			streamName := "/" + uuid.New().String()

			err := client.WriteEventsWithPreconditions([]interface{}{
				eventsourcingdb.NewIsStreamPristinePrecondition(streamName),
			}, []eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "1"}),
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "2"}),
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "3"}),
			})

			assert.NoError(t, err)
		})

		t.Run("returns an error if the stream is not pristine.", func(t *testing.T) {
			client := eventsourcingdb.NewClient(baseURLWithoutAuthorization)
			streamName := "/" + uuid.New().String()

			err := client.WriteEvents([]eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "1"}),
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "2"}),
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "3"}),
			})

			assert.NoError(t, err)

			err = client.WriteEventsWithPreconditions([]interface{}{
				eventsourcingdb.NewIsStreamPristinePrecondition(streamName),
			}, []eventsourcingdb.EventCandidate{
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "4"}),
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "5"}),
				eventsourcingdb.NewEventCandidate(streamName, "testEvent", testPayload{TestData: "6"}),
			})

			assert.Error(t, err)
		})
	})

	t.Run("when using the IsStreamOnEventIDPrecondition", func(t *testing.T) {
		t.Run("writes events if the last event in the stream has the given eventID.", func(t *testing.T) {
			// TODO: We can't read the last eventID yet, so we can't implement the test yet.
		})

		t.Run("returns an error if the last event in the stream does not have the given eventID.", func(t *testing.T) {
			// TODO: We can't read the last eventID yet, so we can't implement the test yet.
		})
	})
}
