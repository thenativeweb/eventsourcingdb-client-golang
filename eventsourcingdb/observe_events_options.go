package eventsourcingdb

type ObserveIfEventIsMissing string

const (
	ObserveIfEventIsMissingWaitForEvent   ObserveIfEventIsMissing = "wait-for-event"
	ObserveIfEventIsMissingReadEverything ObserveIfEventIsMissing = "read-everything"
)

type ObserveFromLatestEvent struct {
	Subject          string
	Type             string
	IfEventIsMissing ObserveIfEventIsMissing
}

type ObserveEventsOptions struct {
	Recursive       bool
	LowerBound      *Bound
	FromLatestEvent *ObserveFromLatestEvent
}
