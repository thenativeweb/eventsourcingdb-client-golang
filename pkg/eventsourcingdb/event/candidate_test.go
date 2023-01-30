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

func TestCandidate_Validate(t *testing.T) {
	t.Run("Returns an error if the source is malformed.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "$%&/(",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct{}{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event source '$%&/(': source must be a valid URI")
	})

	t.Run("Returns an error if the subject is malformed.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "barbaz",
				Type:    "invalid.foobar.event",
			},
			Data: struct{}{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event subject 'barbaz': subject must be an absolute, slash-separated path")
	})

	t.Run("Returns an error if the type is malformed.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid",
			},
			Data: struct{}{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event type 'invalid': type must be a reverse domain name")
	})

	t.Run("Returns an error if the data is not a struct.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: nil,
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: data must be a struct, but received 'invalid'")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: []string{"foo", "bar"},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: data must be a struct, but received 'slice'")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: "foobar",
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: data must be a struct, but received 'string'")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: &struct{}{},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: data must be a struct, but received 'ptr'")
	})

	t.Run("Returns an error if the data contains private fields.", func(t *testing.T) {
		type TestData struct {
			private string
		}
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: TestData{private: "foo"},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: unexported field 'private' at path '[root element]' is not supported, data must only contain exported fields")

	})
}
