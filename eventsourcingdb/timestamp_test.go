package eventsourcingdb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
)

func TestNewTimestamp(t *testing.T) {
	now := time.Now()

	tests := []struct {
		input    time.Time
		expected eventsourcingdb.Timestamp
	}{
		{input: now, expected: eventsourcingdb.Timestamp{Time: now.UTC()}},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, eventsourcingdb.NewTimestamp(test.input))
	}
}
