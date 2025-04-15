package eventsourcingdb

import "github.com/thenativeweb/eventsourcingdb-client-golang/internal"

type Order string

func OrderChronological() *Order {
	return internal.Ptr(Order("chronological"))
}

func OrderAntichronological() *Order {
	return internal.Ptr(Order("antichronological"))
}

type ReadIfEventIsMissing string

const (
	ReadNothingIfEventIsMissing    ReadIfEventIsMissing = "read-nothing"
	ReadEverythingIfEventIsMissing ReadIfEventIsMissing = "read-everything"
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
