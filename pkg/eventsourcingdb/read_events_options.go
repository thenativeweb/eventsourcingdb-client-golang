package eventsourcingdb

import (
	"errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
)

type IfEventIsMissingDuringRead string

const (
	ReadNothing    IfEventIsMissingDuringRead = "read-nothing"
	ReadEverything IfEventIsMissingDuringRead = "read-everything"
)

type ReadRecursivelyOption func() bool

func ReadRecursively() ReadRecursivelyOption {
	return func() bool {
		return true
	}
}

func ReadNonRecursively() ReadRecursivelyOption {
	return func() bool {
		return false
	}
}

type readFromLatestEvent struct {
	Subject          string                     `json:"subject"`
	Type             string                     `json:"type"`
	IfEventIsMissing IfEventIsMissingDuringRead `json:"ifEventIsMissing"`
}

type readEventsOptions struct {
	Recursive       bool                 `json:"recursive"`
	Chronological   *bool                `json:"chronological,omitempty"`
	LowerBoundID    *string              `json:"lowerBoundId,omitempty"`
	UpperBoundID    *string              `json:"upperBoundId,omitempty"`
	FromLatestEvent *readFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

type ReadEventsOption struct {
	apply func(options *readEventsOptions) error
	name  string
}

func ReadChronologically() ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			value := true
			options.Chronological = &value

			return nil
		},
		name: "ReadChronologically",
	}
}

func ReadReversedChronologically() ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			value := false
			options.Chronological = &value

			return nil
		},
		name: "ReadReversedChronologically",
	}
}

func ReadFromLowerBoundID(lowerBoundID string) ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			if options.FromLatestEvent != nil {
				return errors.New("ReadFromLowerBoundID and ReadFromLatestEvent are mutually exclusive")
			}

			options.LowerBoundID = &lowerBoundID

			return nil
		},
		name: "ReadFromLowerBoundID",
	}
}

func ReadUntilUpperBoundID(upperBoundID string) ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			options.UpperBoundID = &upperBoundID

			return nil
		},
		name: "ReadUntilUpperBoundID",
	}
}

func ReadFromLatestEvent(subject, eventType string, ifEventIsMissing IfEventIsMissingDuringRead) ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			if options.LowerBoundID != nil {
				return errors.New("ReadFromLowerBoundID and ReadFromLatestEvent are mutually exclusive")
			}
			if err := event.ValidateSubject(subject); err != nil {
				return err
			}
			if err := event.ValidateType(eventType); err != nil {
				return err
			}

			options.FromLatestEvent = &readFromLatestEvent{
				Subject:          subject,
				Type:             eventType,
				IfEventIsMissing: ifEventIsMissing,
			}

			return nil
		},
		name: "ReadFromLatestEvent",
	}
}
