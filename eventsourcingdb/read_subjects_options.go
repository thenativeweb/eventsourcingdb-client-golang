package eventsourcingdb

import "github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb/event"

type ReadSubjectsOption struct {
	apply func(options *readSubjectsRequestBody) error
	name  string
}

func BaseSubject(baseSubject string) ReadSubjectsOption {
	return ReadSubjectsOption{
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
