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
	rawJsonPreconditions := make(
		[]json.RawMessage,
		0,
		len(preconditions.isStreamPristinePreconditions)+len(preconditions.isStreamOnEventIDPrecondition),
	)

	for _, precondition := range preconditions.isStreamPristinePreconditions {
		rawJsonPrecondition, err := json.Marshal(precondition)

		if err != nil {
			return []byte{}, err
		}

		rawJsonPreconditions = append(rawJsonPreconditions, rawJsonPrecondition)
	}

	for _, precondition := range preconditions.isStreamOnEventIDPrecondition {
		rawJsonPrecondition, err := json.Marshal(precondition)

		if err != nil {
			return []byte{}, err
		}

		rawJsonPreconditions = append(rawJsonPreconditions, rawJsonPrecondition)
	}

	return json.Marshal(rawJsonPreconditions)
}
