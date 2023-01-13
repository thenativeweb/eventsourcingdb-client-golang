package eventsourcingdb

import "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"

type ReadSubjectOption func(options *readSubjectsRequestBody) error

func BaseSubject(baseSubject string) ReadSubjectOption {
	return func(options *readSubjectsRequestBody) error {
		if err := event.ValidateSubject(baseSubject); err != nil {
			return err
		}

		options.BaseSubject = baseSubject

		return nil
	}
}
