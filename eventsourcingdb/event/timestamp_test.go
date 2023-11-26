package event_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb/event"
)

func TestNewTimestamp(t *testing.T) {
	now := time.Now()

	tests := []struct {
		input    time.Time
		expected event.Timestamp
	}{
		{input: now, expected: event.Timestamp{Time: now.UTC()}},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, event.NewTimestamp(test.input))
	}
}
