package eventsourcingdb

type Order string

func OrderChronological() *Order {
	return new(Order("chronological"))
}

func OrderAntichronological() *Order {
	return new(Order("antichronological"))
}

type ReadIfEventIsMissing string

const (
	ReadIfEventIsMissingReadNothing    ReadIfEventIsMissing = "read-nothing"
	ReadIfEventIsMissingReadEverything ReadIfEventIsMissing = "read-everything"
)

type ReadFromLatestEvent struct {
	Subject          string
	Type             string
	IfEventIsMissing ReadIfEventIsMissing
}

type ReadEventsOptions struct {
	Recursive       bool
	Order           *Order
	LowerBound      *Bound
	UpperBound      *Bound
	FromLatestEvent *ReadFromLatestEvent
}
