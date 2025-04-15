package eventsourcingdb

type ObserveIfEventIsMissing string

const (
	WaitForEventIfEventIsMissing      ObserveIfEventIsMissing = "wait-for-event"
	ObserveEverythingIfEventIsMissing ObserveIfEventIsMissing = "read-everything"
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
