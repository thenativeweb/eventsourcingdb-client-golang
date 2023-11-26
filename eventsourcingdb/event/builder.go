package event

type Source struct {
	source string
}

func (eventSource Source) NewEvent(subject, eventType string, data Data, options ...CandidateTransformer) Candidate {
	return NewCandidate(eventSource.source, subject, eventType, data, options...)
}

func NewSource(source string) Source {
	return Source{source}
}
