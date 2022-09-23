package eventsourcingdb

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

type EventCandidate struct {
	Metadata EventCandidateMetadata
	Data     interface{}
}

func NewEventCandidate(streamName, name string, data interface{}) EventCandidate {
	return EventCandidate{
		NewEventCandidateMetadata(streamName, name),
		data,
	}
}
