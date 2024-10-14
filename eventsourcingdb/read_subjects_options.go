package eventsourcingdb


type ReadSubjectsOption struct {
	apply func(options *readSubjectsRequestBody) error
	name  string
}

func BaseSubject(baseSubject string) ReadSubjectsOption {
	return ReadSubjectsOption{
		apply: func(options *readSubjectsRequestBody) error {
			if err := validateSubject(baseSubject); err != nil {
				return err
			}

			options.BaseSubject = baseSubject

			return nil
		},
		name: "BaseSubject",
	}
}
