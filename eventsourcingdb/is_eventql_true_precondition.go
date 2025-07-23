package eventsourcingdb

type isEventQLTruePrecondition struct {
	query string
}

func (isEventQLTruePrecondition) discriminator() {}

func NewIsEventQLTruePrecondition(query string) Precondition {
	return isEventQLTruePrecondition{
		query,
	}
}

func (p isEventQLTruePrecondition) Query() string {
	return p.query
}
