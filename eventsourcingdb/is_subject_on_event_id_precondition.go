package eventsourcingdb

type isSubjectOnEventIDPrecondition struct {
	subject string
	eventID string
	options *PreconditionOptions
}

func (isSubjectOnEventIDPrecondition) discriminator() {}

// NewIsSubjectOnEventIDPrecondition returns a precondition that holds if the
// latest event of the given subject has the given ID, or, if the given
// options make it recursive, if the latest event among the subject and all
// subjects below it has that ID.
func NewIsSubjectOnEventIDPrecondition(subject string, eventID string, options ...PreconditionOptions) Precondition {
	return isSubjectOnEventIDPrecondition{
		subject: subject,
		eventID: eventID,
		options: preconditionOptionsOf(options),
	}
}

func (p isSubjectOnEventIDPrecondition) Subject() string {
	return p.subject
}

func (p isSubjectOnEventIDPrecondition) EventID() string {
	return p.eventID
}
