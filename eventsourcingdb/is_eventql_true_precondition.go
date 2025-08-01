package eventsourcingdb

type isEventQLQueryTruePrecondition struct {
	query string
}

func (isEventQLQueryTruePrecondition) discriminator() {}

func NewIsEventQLQueryTruePrecondition(query string) Precondition {
	return isEventQLQueryTruePrecondition{
		query,
	}
}

func (p isEventQLQueryTruePrecondition) Query() string {
	return p.query
}
