package eventsourcingdb

type isSubjectPopulatedPrecondition struct {
	subject string
}

func (isSubjectPopulatedPrecondition) discriminator() {}

func NewIsSubjectPopulatedPrecondition(subject string) Precondition {
	return isSubjectPopulatedPrecondition{
		subject,
	}
}

func (p isSubjectPopulatedPrecondition) Subject() string {
	return p.subject
}
