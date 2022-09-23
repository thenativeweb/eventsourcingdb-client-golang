package eventsourcingdb

type IsStreamPristinePrecondition struct {
	StreamName string
}

type IsStreamOnEventIDPrecondition struct {
	StreamName string
	EventID    int
}
