package eventsourcingdb

import (
	"fmt"
)

type EventCandidate struct {
	Source      string  `json:"source"`
	Subject     string  `json:"subject"`
	Type        string  `json:"type"`
	Data        any     `json:"data"`
	TraceParent *string `json:"traceparent,omitempty"`
	TraceState  *string `json:"tracestate,omitempty"`
}

type EventOptions func(candidate *EventCandidate)

func WithTraceParent(traceParent string) EventOptions {
	return func(candidate *EventCandidate) {
		candidate.TraceParent = &traceParent
	}
}

func WithTraceState(traceState string) EventOptions {
	return func(candidate *EventCandidate) {
		candidate.TraceState = &traceState
	}
}

func NewEventCandidate(
	source string,
	subject string,
	eventType string,
	data any,
	options ...EventOptions,
) EventCandidate {
	candidate := EventCandidate{
		Source:  source,
		Subject: subject,
		Type:    eventType,
		Data:    data,
	}

	for _, option := range options {
		option(&candidate)
	}

	return candidate
}

func (candidate EventCandidate) Validate() error {
	if err := validateSource(candidate.Source); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	if err := validateSubject(candidate.Subject); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	if err := validateEventType(candidate.Type); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	if candidate.TraceState != nil && candidate.TraceParent == nil {
		return fmt.Errorf("event candidate failed to validate: traceparent is required when tracestate is provided")
	}

	return nil
}
