package eventsourcingdb

import "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"

type ReadSubjectOption struct {
	apply func(options *readSubjectsRequestBody) error
	name  string
}

func BaseSubject(baseSubject string) ReadSubjectOption {
	return ReadSubjectOption{
		apply: func(options *readSubjectsRequestBody) error {
			if err := event.ValidateSubject(baseSubject); err != nil {
				return err
			}

			options.BaseSubject = baseSubject

			return nil
		},
		name: "BaseSubject",
	}
}
