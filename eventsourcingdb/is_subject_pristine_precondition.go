package eventsourcingdb

type isSubjectPristinePrecondition struct {
	subject string
	options *PreconditionOptions
}

func (isSubjectPristinePrecondition) discriminator() {}

// NewIsSubjectPristinePrecondition returns a precondition that holds if the
// given subject has no event yet, or, if the given options make it recursive,
// if neither the subject nor any subject below it has one.
func NewIsSubjectPristinePrecondition(subject string, options ...PreconditionOptions) Precondition {
	return isSubjectPristinePrecondition{
		subject: subject,
		options: preconditionOptionsOf(options),
	}
}

func (p isSubjectPristinePrecondition) Subject() string {
	return p.subject
}
