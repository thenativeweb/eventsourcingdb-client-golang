package eventsourcingdb

import "encoding/json"

type EventContext struct {
	EventCandidateContext
	SpecVersion     string    `json:"specversion"`
	ID              string    `json:"id"`
	Time            Timestamp `json:"time"`
	DataContentType string    `json:"datacontenttype"`
	PredecessorHash string    `json:"predecessorhash"`
}

type Event struct {
	EventContext
	Data json.RawMessage `json:"data"`
}

func (eventSource Source) NewEvent(subject, eventType string, data Data, options ...EventCandidateTransformer) EventCandidate {
	return NewEventCandidate(eventSource.source, subject, eventType, data, options...)
}