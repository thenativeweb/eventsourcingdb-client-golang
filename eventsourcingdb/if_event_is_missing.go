package eventsourcingdb

type IfEventIsMissingDuringRead string

const (
	IfEventIsMissingDuringReadReadEverything IfEventIsMissingDuringRead = "read-everything"
	IfEventIsMissingDuringReadReadNothing    IfEventIsMissingDuringRead = "read-nothing"
)

type IfEventIsMissingDuringObserve string

const (
	IfEventIsMissingDuringObserveReadEverything IfEventIsMissingDuringObserve = "read-everything"
	IfEventIsMissingDuringObserveWaitForEvent   IfEventIsMissingDuringObserve = "wait-for-event"
)
