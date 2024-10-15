package eventsourcingdb

import (
	"fmt"
	"net/url"
)

type Source struct {
	source string
}

func NewSource(source string) Source {
	return Source{source}
}

func validateSource(source string) error {
	if _, err := url.Parse(source); err != nil {
		return fmt.Errorf("malformed event source '%s': source must be a valid URI", source)
	}

	return nil
}

func (eventSource Source) NewEvent(subject, eventType string, data any) EventCandidate {
	return NewEventCandidate(eventSource.source, subject, eventType, data)
}
