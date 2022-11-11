package eventsourcingdb

import (
	"encoding/json"
)

const (
	isStreamPristine  = "isStreamPristine"
	isStreamOnEventID = "isStreamOnEventId"
)

type isStreamPristinePrecondition struct {
	StreamName string `json:"streamName"`
}

type isStreamOnEventIDPrecondition struct {
	StreamName string `json:"streamName"`
	EventID    int    `json:"eventId"`
}

type precondition[TContent any] struct {
	Type    string   `json:"type"`
	Payload TContent `json:"payload"`
}

type Preconditions struct {
	isStreamPristinePreconditions []precondition[isStreamPristinePrecondition]
	isStreamOnEventIDPrecondition []precondition[isStreamOnEventIDPrecondition]
}

func NewPreconditions() *Preconditions {
	return &Preconditions{}
}

func (preconditions *Preconditions) IsStreamPristine(streamName string) *Preconditions {
	preconditions.isStreamPristinePreconditions = append(
		preconditions.isStreamPristinePreconditions,
		precondition[isStreamPristinePrecondition]{
			Type: isStreamPristine,
			Payload: isStreamPristinePrecondition{
				StreamName: streamName,
			},
		},
	)

	return preconditions
}

func (preconditions *Preconditions) IsStreamOnEventID(streamName string, eventID int) *Preconditions {
	preconditions.isStreamOnEventIDPrecondition = append(
		preconditions.isStreamOnEventIDPrecondition,
		precondition[isStreamOnEventIDPrecondition]{
			Type: isStreamOnEventID,
			Payload: isStreamOnEventIDPrecondition{
				StreamName: streamName,
				EventID:    eventID,
			},
		},
	)

	return preconditions
}

func (preconditions *Preconditions) MarshalJSON() ([]byte, error) {
	rawJSONPreconditions := make(
		[]json.RawMessage,
		0,
		len(preconditions.isStreamPristinePreconditions)+len(preconditions.isStreamOnEventIDPrecondition),
	)

	for _, precondition := range preconditions.isStreamPristinePreconditions {
		rawJSONPrecondition, err := json.Marshal(precondition)

		if err != nil {
			return []byte{}, err
		}

		rawJSONPreconditions = append(rawJSONPreconditions, rawJSONPrecondition)
	}

	for _, precondition := range preconditions.isStreamOnEventIDPrecondition {
		rawJSONPrecondition, err := json.Marshal(precondition)

		if err != nil {
			return []byte{}, err
		}

		rawJSONPreconditions = append(rawJSONPreconditions, rawJSONPrecondition)
	}

	return json.Marshal(rawJSONPreconditions)
}
