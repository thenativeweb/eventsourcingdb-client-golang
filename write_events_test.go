package eventsourcingdb_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"testing"
)

type testPayload struct {
	TestData string `json:"testData"`
}

func TestWriteEvents(t *testing.T) {
	t.Run("writes events to the database.", func(t *testing.T) {
		client := eventsourcingdb.NewClient(baseUrl)

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate("/testStream", "testEvent", testPayload{TestData: "1"}),
			eventsourcingdb.NewEventCandidate("/testStream", "testEvent", testPayload{TestData: "2"}),
			eventsourcingdb.NewEventCandidate("/testStream", "testEvent", testPayload{TestData: "3"}),
			eventsourcingdb.NewEventCandidate("/testStream", "testEvent", testPayload{TestData: "4"}),
		})

		assert.NoError(t, err)
	})

	t.Run("fails when trying to write events to a nonexistent database.", func(t *testing.T) {
		client := eventsourcingdb.NewClient("schwibbedi.invalid")

		err := client.WriteEvents([]eventsourcingdb.EventCandidate{
			eventsourcingdb.NewEventCandidate("/testStream", "testEvent", testPayload{TestData: "1"}),
			eventsourcingdb.NewEventCandidate("/testStream", "testEvent", testPayload{TestData: "2"}),
			eventsourcingdb.NewEventCandidate("/testStream", "testEvent", testPayload{TestData: "3"}),
			eventsourcingdb.NewEventCandidate("/testStream", "testEvent", testPayload{TestData: "4"}),
		})

		assert.Error(t, err)
	})
}
