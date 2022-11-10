package eventsourcingdb

type IfEventIsMissingDuringObserve string

const (
	ReadNothingIfEventIsMissingDuringObserve  IfEventIsMissingDuringObserve = "read-nothing"
	WaitForEventIfEventIsMissingDuringObserve IfEventIsMissingDuringObserve = "wait-for-event"
)

type ObserveFromLatestEvent struct {
	StreamName       string                        `json:"streamName"`
	EventName        string                        `json:"eventName"`
	IfEventIsMissing IfEventIsMissingDuringObserve `json:"ifEventIsMissing"`
}

type ObserveEventsOptions struct {
	OptionRecursive       bool                    `json:"recursive"`
	OptionEventNames      *[]string               `json:"eventNames,omitempty"`
	OptionLowerBoundID    *int                    `json:"lowerBoundId,omitempty"`
	OptionFromLatestEvent *ObserveFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

func NewObserveEventsOptions(recursive bool) ObserveEventsOptions {
	return ObserveEventsOptions{
		OptionRecursive: recursive,
	}
}

func (options ObserveEventsOptions) EventNames(eventNames []string) ObserveEventsOptions {
	options.OptionEventNames = &eventNames

	return options
}

func (options ObserveEventsOptions) LowerBoundID(lowerBoundID int) ObserveEventsOptions {
	options.OptionLowerBoundID = &lowerBoundID

	return options
}

func (options ObserveEventsOptions) FromLatestEvent(fromLatestEvent ObserveFromLatestEvent) ObserveEventsOptions {
	options.OptionFromLatestEvent = &fromLatestEvent

	return options
}
