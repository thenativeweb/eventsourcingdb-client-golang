package eventsourcingdb

import "encoding/json"

type EventCandidateMetadata struct {
	StreamName string
	Name       string
}

func NewEventCandidateMetadata(streamName, name string) EventCandidateMetadata {
	return EventCandidateMetadata{
		streamName,
		name,
	}
}

type EventCandidate interface {
	Metadata() EventCandidateMetadata
	Data() json.RawMessage
}

type eventCandidate[TData any] struct {
	metadata EventCandidateMetadata
	data     TData
}

func (candidate eventCandidate[TData]) Metadata() EventCandidateMetadata {
	return candidate.metadata
}

func (candidate eventCandidate[TData]) Data() json.RawMessage {
	data, err := json.Marshal(candidate.data)
	if err != nil {
		panic(err)
	}

	return data
}

func NewEventCandidate[TData any](streamName, name string, data TData) EventCandidate {
	return eventCandidate[TData]{
		NewEventCandidateMetadata(streamName, name),
		data,
	}
}
