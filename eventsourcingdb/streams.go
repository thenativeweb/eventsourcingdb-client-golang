package eventsourcingdb

import (
	"encoding/json"
	"fmt"
)

// errStreamBrokeOff reports that a stream ended before it was complete: a
// stream of the v2 API that ends on its own before its end item, and a stream
// that never ends, i.e. one of observed events, at all, without an error. The
// server ends a stream so only after a failure it does not report, e.g. one it
// merely logs, so what the stream handed out may be incomplete. A server that
// shuts down reports that with an error item instead.
func errStreamBrokeOff(action string) error {
	return fmt.Errorf("failed to %s, the stream broke off", action)
}

// yieldEventOf hands the event that a line or an item of a stream carries to
// yield, and returns whether to go on reading. A line or item of any other
// type ends the stream with an error.
func yieldEventOf(lineType string, payload json.RawMessage, yield func(Event, error) bool) bool {
	if lineType != "event" {
		yield(Event{}, errUnsupportedLineType(lineType))
		return false
	}

	var event Event
	err := json.Unmarshal(payload, &event)
	if err != nil {
		yield(Event{}, err)
		return false
	}

	return yield(event, nil)
}

// streamedSubject is a subject as a line or an item of a stream carries it.
type streamedSubject struct {
	Subject string `json:"subject"`
}

// errUnsupportedLineType reports a line or an item of a stream of a type that
// the stream does not have.
func errUnsupportedLineType(lineType string) error {
	return fmt.Errorf("failed to handle unsupported line type: %s", lineType)
}
