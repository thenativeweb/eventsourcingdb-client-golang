package event

type Source struct {
	source string
}

func (eventSource Source) NewEvent(subject, eventType string, data any) Candidate {
	return NewCandidate(eventSource.source, subject, eventType, data)
}

func NewSource(source string) Source {
	return Source{source}
}
