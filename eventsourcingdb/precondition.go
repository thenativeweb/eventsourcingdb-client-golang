package eventsourcingdb

type Precondition interface {
	discriminator()
}

type isSubjectPristinePrecondition struct {
	subject string
}

func (isSubjectPristinePrecondition) discriminator() {}

func NewIsSubjectPristinePrecondition(subject string) Precondition {
	return isSubjectPristinePrecondition{
		subject,
	}
}

func (p isSubjectPristinePrecondition) Subject() string {
	return p.subject
}

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
