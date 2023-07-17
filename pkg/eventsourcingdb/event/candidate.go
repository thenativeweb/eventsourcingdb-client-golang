package event

import (
	"fmt"
)

type CandidateContext struct {
	Source         string          `json:"source"`
	Subject        string          `json:"subject"`
	Type           string          `json:"type"`
	TracingContext *TracingContext `json:"tracingContext"`
}

type Candidate struct {
	CandidateContext
	Data Data `json:"data"`
}

func NewCandidate(
	source string,
	subject string,
	eventType string,
	data Data,
	tracingContext *TracingContext,
) Candidate {
	return Candidate{
		CandidateContext{
			Source:         source,
			Subject:        subject,
			Type:           eventType,
			TracingContext: tracingContext,
		},
		data,
	}
}

func (candidate Candidate) Validate() error {
	if err := ValidateSource(candidate.Source); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	if err := ValidateSubject(candidate.Subject); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	if err := ValidateType(candidate.Type); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	return nil
}
