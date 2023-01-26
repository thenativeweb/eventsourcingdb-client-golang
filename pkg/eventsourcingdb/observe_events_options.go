package eventsourcingdb

import (
	"errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"strconv"
)

type IfEventIsMissingDuringObserve string

const (
	ObserveNothing IfEventIsMissingDuringObserve = "read-nothing"
	WaitForEvent   IfEventIsMissingDuringObserve = "wait-for-event"
)

type ObserveRecursivelyOption func() bool

func ObserveRecursively() ObserveRecursivelyOption {
	return func() bool {
		return true
	}
}

func ObserveNonRecursively() ObserveRecursivelyOption {
	return func() bool {
		return false
	}
}

type observeFromLatestEvent struct {
	Subject          string                        `json:"subject"`
	Type             string                        `json:"type"`
	IfEventIsMissing IfEventIsMissingDuringObserve `json:"ifEventIsMissing"`
}

type observeEventsOptions struct {
	Recursive       bool                    `json:"recursive"`
	LowerBoundID    *string                 `json:"lowerBoundId,omitempty"`
	FromLatestEvent *observeFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

type ObserveEventsOption struct {
	apply func(options *observeEventsOptions) error
	name  string
}

func ObserveFromLowerBoundID(lowerBoundID string) ObserveEventsOption {
	return ObserveEventsOption{
		apply: func(options *observeEventsOptions) error {
			if options.FromLatestEvent != nil {
				return errors.New("ObserveFromLowerBoundID and ObserveFromLatestEvent are mutually exclusive")
			}

			parsedLowerBoundID, err := strconv.Atoi(lowerBoundID)
			if err != nil {
				return errors.New("lowerBoundID must contain an integer")
			}
			if parsedLowerBoundID < 0 {
				return errors.New("lowerBoundID must be 0 or greater")
			}

			options.LowerBoundID = &lowerBoundID

			return nil
		},
		name: "ObserveFromLowerBoundID",
	}
}

func ObserveFromLatestEvent(subject, eventType string, ifEventIsMissing IfEventIsMissingDuringObserve) ObserveEventsOption {
	return ObserveEventsOption{
		apply: func(options *observeEventsOptions) error {
			if options.LowerBoundID != nil {
				return errors.New("ObserveFromLowerBoundID and ObserveFromLatestEvent are mutually exclusive")
			}
			if err := event.ValidateSubject(subject); err != nil {
				return err
			}
			if err := event.ValidateType(eventType); err != nil {
				return err
			}

			options.FromLatestEvent = &observeFromLatestEvent{
				Subject:          subject,
				Type:             eventType,
				IfEventIsMissing: ifEventIsMissing,
			}

			return nil
		},
		name: "ObserveFromLatestEvent",
	}
}
