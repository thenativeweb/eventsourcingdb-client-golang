package eventsourcingdb

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
