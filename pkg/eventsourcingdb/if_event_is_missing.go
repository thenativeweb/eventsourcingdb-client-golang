package eventsourcingdb

type IfEventIsMissing string

const (
	ReadNothing    IfEventIsMissing = "read-nothing"
	ReadEverything IfEventIsMissing = "read-everything"
	WaitForEvent   IfEventIsMissing = "wait-for-event"
)
