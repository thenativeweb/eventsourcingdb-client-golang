package eventsourcingdb

import (
	"fmt"
)

type EventCandidateContext struct {
	Source      string  `json:"source"`
	Subject     string  `json:"subject"`
	Type        string  `json:"type"`
	TraceParent *string `json:"traceparent,omitempty"`
	TraceState  *string `json:"tracestate,omitempty"`
}

type EventCandidate struct {
	EventCandidateContext
	Data Data `json:"data"`
}

type EventCandidateTransformer func(candidate *EventCandidate)

func WithTraceParent(traceParent string) EventCandidateTransformer {
	return func(candidate *EventCandidate) {
		candidate.TraceParent = &traceParent
	}
}

func WithTraceState(traceState string) EventCandidateTransformer {
	return func(candidate *EventCandidate) {
		candidate.TraceParent = &traceState
	}
}

func NewEventCandidate(
	source string,
	subject string,
	eventType string,
	data Data,
	transformers ...EventCandidateTransformer,
) EventCandidate {
	candidate := EventCandidate{
		EventCandidateContext{
			Source:  source,
			Subject: subject,
			Type:    eventType,
		},
		data,
	}

	for _, transformer := range transformers {
		transformer(&candidate)
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
