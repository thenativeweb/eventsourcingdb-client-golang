package eventsourcingdb

import (
	"net/url"
)

type EventCandidateContext struct {
	Subject string
	Type    string
	Source  url.URL
}

func NewEventCandidateContext(subject, name string, source url.URL) EventCandidateContext {
	return EventCandidateContext{
		subject,
		name,
		source,
	}
}

type EventCandidate struct {
	EventCandidateContext
	Data any
}

func NewEventCandidate(subject, name string, source url.URL, data any) EventCandidate {
	return EventCandidate{
		NewEventCandidateContext(subject, name, source),
		data,
	}
}
