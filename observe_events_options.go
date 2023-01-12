package eventsourcingdb

type IfEventIsMissingDuringObserve string

const (
	ReadNothingIfEventIsMissingDuringObserve  IfEventIsMissingDuringObserve = "read-nothing"
	WaitForEventIfEventIsMissingDuringObserve IfEventIsMissingDuringObserve = "wait-for-event"
)

type ObserveFromLatestEvent struct {
	Subject          string                        `json:"subject"`
	Type             string                        `json:"type"`
	IfEventIsMissing IfEventIsMissingDuringObserve `json:"ifEventIsMissing"`
}

type ObserveEventsOptions struct {
	OptionRecursive       bool                    `json:"recursive"`
	OptionLowerBoundID    *int                    `json:"lowerBoundId,omitempty"`
	OptionFromLatestEvent *ObserveFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

func NewObserveEventsOptions(recursive bool) ObserveEventsOptions {
	return ObserveEventsOptions{
		OptionRecursive: recursive,
	}
}

func (options ObserveEventsOptions) LowerBoundID(lowerBoundID int) ObserveEventsOptions {
	options.OptionLowerBoundID = &lowerBoundID

	return options
}

func (options ObserveEventsOptions) FromLatestEvent(fromLatestEvent ObserveFromLatestEvent) ObserveEventsOptions {
	options.OptionFromLatestEvent = &fromLatestEvent

	return options
}
