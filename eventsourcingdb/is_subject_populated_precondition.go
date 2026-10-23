package eventsourcingdb

type isSubjectPopulatedPrecondition struct {
	subject string
	options *PreconditionOptions
}

func (isSubjectPopulatedPrecondition) discriminator() {}

// NewIsSubjectPopulatedPrecondition returns a precondition that holds if the
// given subject has at least one event, or, if the given options make it
// recursive, if the subject or any subject below it has one.
func NewIsSubjectPopulatedPrecondition(subject string, options ...PreconditionOptions) Precondition {
	return isSubjectPopulatedPrecondition{
		subject: subject,
		options: preconditionOptionsOf(options),
	}
}

func (p isSubjectPopulatedPrecondition) Subject() string {
	return p.subject
}
