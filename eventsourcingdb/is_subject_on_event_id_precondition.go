package eventsourcingdb

type isSubjectOnEventIDPrecondition struct {
	subject string
	eventID string
}

func (isSubjectOnEventIDPrecondition) discriminator() {}

func NewIsSubjectOnEventIDPrecondition(subject string, eventID string) Precondition {
	return isSubjectOnEventIDPrecondition{
		subject,
		eventID,
	}
}

func (p isSubjectOnEventIDPrecondition) Subject() string {
	return p.subject
}

func (p isSubjectOnEventIDPrecondition) EventID() string {
	return p.eventID
}
