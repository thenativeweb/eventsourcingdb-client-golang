package eventsourcingdb

import (
	"encoding/json"
)

const (
	isSubjectPristine  = "isSubjectPristine"
	isSubjectOnEventID = "isSubjectOnEventId"
)

type isSubjectPristinePrecondition struct {
	Subject string `json:"subject"`
}

type isSubjectOnEventIDPrecondition struct {
	Subject string `json:"subject"`
	EventID string `json:"eventId"`
}

type precondition[TContent any] struct {
	Type    string   `json:"type"`
	Payload TContent `json:"payload"`
}

type PreconditionsBody struct {
	isSubjectPristinePreconditions  []precondition[isSubjectPristinePrecondition]
	isSubjectOnEventIDPreconditions []precondition[isSubjectOnEventIDPrecondition]
}

type Precondition func(preconditions *PreconditionsBody)

func Preconditions(preconditions ...Precondition) *PreconditionsBody {
	preconditionsBody := &PreconditionsBody{}

	for _, addPrecondition := range preconditions {
		addPrecondition(preconditionsBody)
	}

	return preconditionsBody
}

func IsSubjectPristine(subject string) Precondition {
	return func(preconditions *PreconditionsBody) {
		preconditions.isSubjectPristinePreconditions = append(
			preconditions.isSubjectPristinePreconditions,
			precondition[isSubjectPristinePrecondition]{
				Type: isSubjectPristine,
				Payload: isSubjectPristinePrecondition{
					Subject: subject,
				},
			},
		)
	}
}

func IsSubjectOnEventID(subject string, eventID string) Precondition {
	return func(preconditions *PreconditionsBody) {
		preconditions.isSubjectOnEventIDPreconditions = append(
			preconditions.isSubjectOnEventIDPreconditions,
			precondition[isSubjectOnEventIDPrecondition]{
				Type: isSubjectOnEventID,
				Payload: isSubjectOnEventIDPrecondition{
					Subject: subject,
					EventID: eventID,
				},
			},
		)
	}
}

func (preconditions *PreconditionsBody) MarshalJSON() ([]byte, error) {
	rawJSONPreconditions := make(
		[]json.RawMessage,
		0,
		len(preconditions.isSubjectPristinePreconditions)+len(preconditions.isSubjectOnEventIDPreconditions),
	)

	for _, precondition := range preconditions.isSubjectPristinePreconditions {
		rawJSONPrecondition, err := json.Marshal(precondition)
		if err != nil {
			return []byte{}, err
		}

		rawJSONPreconditions = append(rawJSONPreconditions, rawJSONPrecondition)
	}

	for _, precondition := range preconditions.isSubjectOnEventIDPreconditions {
		rawJSONPrecondition, err := json.Marshal(precondition)
		if err != nil {
			return []byte{}, err
		}

		rawJSONPreconditions = append(rawJSONPreconditions, rawJSONPrecondition)
	}

	return json.Marshal(rawJSONPreconditions)
}
