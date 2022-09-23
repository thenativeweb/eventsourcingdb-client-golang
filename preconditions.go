package eventsourcingdb

type IsStreamPristinePrecondition struct {
	StreamName string
}

func NewIsStreamPristinePrecondition(streamName string) IsStreamPristinePrecondition {
	return IsStreamPristinePrecondition{streamName}
}

type IsStreamOnEventIDPrecondition struct {
	StreamName string
	EventID    int
}

func NewIsStreamOnEventIDPrecondition(streamName string, eventID int) IsStreamOnEventIDPrecondition {
	return IsStreamOnEventIDPrecondition{streamName, eventID}
}
