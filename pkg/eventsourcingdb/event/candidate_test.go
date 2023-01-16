package event_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"testing"
	"time"
)

func TestNewCandidate(t *testing.T) {
	tests := []struct {
		timestamp event.Timestamp
		subject   string
		eventType string
		data      event.Data
	}{
		{
			timestamp: event.Timestamp{Time: time.Now()},
			subject:   "/account/user",
			eventType: "registered",
			data:      map[string]interface{}{"username": "jane.doe", "password": "secret"},
		},
	}

	for _, test := range tests {
		createdEvent := event.NewCandidate(events.TestSource, test.subject, test.eventType, test.data)

		assert.Equal(t, test.subject, createdEvent.Subject)
		assert.Equal(t, test.eventType, createdEvent.Type)
		assert.Equal(t, test.data, createdEvent.Data)
	}
}
