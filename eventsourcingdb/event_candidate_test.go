package eventsourcingdb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
)

func TestNewCandidate(t *testing.T) {
	tests := []struct {
		timestamp eventsourcingdb.Timestamp
		subject   string
		eventType string
		data      any
	}{
		{
			timestamp: eventsourcingdb.Timestamp{Time: time.Now()},
			subject:   "/account/user",
			eventType: "registered",
			data:      map[string]any{"username": "jane.doe", "password": "secret"},
		},
	}

	for _, test := range tests {
		createdEvent := eventsourcingdb.NewEventCandidate(events.TestSource, test.subject, test.eventType, test.data)

		assert.Equal(t, test.subject, createdEvent.Subject)
		assert.Equal(t, test.eventType, createdEvent.Type)
		assert.Equal(t, test.data, createdEvent.Data)
	}
}

func TestCandidate_Validate(t *testing.T) {
	t.Run("returns an error if the source is malformed.", func(t *testing.T) {
		err := eventsourcingdb.EventCandidate{
			Source:  "$%&/(",
			Subject: "/foo/bar",
			Type:    "invalid.foobar.event",
			Data:    map[string]any{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event source '$%&/(': source must be a valid URI")
	})

	t.Run("returns an error if the subject is malformed.", func(t *testing.T) {
		err := eventsourcingdb.EventCandidate{
			Source:  "tag:foobar.invalid,2023:service",
			Subject: "barbaz",
			Type:    "invalid.foobar.event",
			Data:    map[string]any{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event subject 'barbaz': subject must be an absolute, slash-separated path")
	})

	t.Run("returns an error if the type is malformed.", func(t *testing.T) {
		err := eventsourcingdb.EventCandidate{
			Source:  "tag:foobar.invalid,2023:service",
			Subject: "/foo/bar",
			Type:    "invalid",
			Data:    map[string]any{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event type 'invalid': type must be a reverse domain name")
	})

	t.Run("returns an error if a tracestate is given but no traceparent.", func(t *testing.T) {
		traceState := "foo=bar"
		err := eventsourcingdb.EventCandidate{
			Source:     "tag:foobar.invalid,2023:service",
			Subject:    "/foo/bar",
			Type:       "invalid.foobar.event",
			TraceState: &traceState,
			Data:       map[string]any{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: traceparent is required when tracestate is provided")
	})
}
