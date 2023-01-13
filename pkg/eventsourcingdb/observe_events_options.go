package eventsourcingdb

type IfEventIsMissingDuringObserve string

const (
	ReadNothingIfEventIsMissingDuringObserve  IfEventIsMissingDuringObserve = "read-nothing"
	WaitForEventIfEventIsMissingDuringObserve IfEventIsMissingDuringObserve = "wait-for-event"
)

type observeFromLatestEvent struct {
	Subject          string                        `json:"subject"`
	Type             string                        `json:"type"`
	IfEventIsMissing IfEventIsMissingDuringObserve `json:"ifEventIsMissing"`
}

type observeEventsOptions struct {
	Recursive       bool                    `json:"recursive"`
	LowerBoundID    *int                    `json:"lowerBoundId,omitempty"`
	FromLatestEvent *observeFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

type ObserveEventsOption func(options *observeEventsOptions)

func ObserveFromLowerBoundID(lowerBoundID int) ObserveEventsOption {
	return func(options *observeEventsOptions) {
		options.LowerBoundID = &lowerBoundID
	}
}

func ObserveFromLatestEvent(subject, eventType string, ifEventIsMissing IfEventIsMissingDuringObserve) ObserveEventsOption {
	return func(options *observeEventsOptions) {
		options.FromLatestEvent = &observeFromLatestEvent{
			Subject:          subject,
			Type:             eventType,
			IfEventIsMissing: ifEventIsMissing,
		}
	}
}
