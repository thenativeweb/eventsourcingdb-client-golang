package eventsourcingdb

import (
	"errors"
	"strconv"

	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb/ifeventismissingduringread"
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
	Subject          string                                      `json:"subject"`
	Type             string                                      `json:"type"`
	IfEventIsMissing ifeventismissingduringread.IfEventIsMissing `json:"ifEventIsMissing"`
}

type readEventsOptions struct {
	Recursive       bool                 `json:"recursive"`
	Order           *string              `json:"order,omitempty"`
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
			value := "chronological"
			options.Order = &value

			return nil
		},
		name: "ReadChronologically",
	}
}

func ReadAntichronologically() ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			value := "antichronological"
			options.Order = &value

			return nil
		},
		name: "ReadAntichronologically",
	}
}

func ReadFromLowerBoundID(lowerBoundID string) ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			if options.FromLatestEvent != nil {
				return errors.New("ReadFromLowerBoundID and ReadFromLatestEvent are mutually exclusive")
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
		name: "ReadFromLowerBoundID",
	}
}

func ReadUntilUpperBoundID(upperBoundID string) ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			parsedUpperBoundID, err := strconv.Atoi(upperBoundID)
			if err != nil {
				return errors.New("upperBoundID must contain an integer")
			}
			if parsedUpperBoundID < 0 {
				return errors.New("upperBoundID must be 0 or greater")
			}

			options.UpperBoundID = &upperBoundID

			return nil
		},
		name: "ReadUntilUpperBoundID",
	}
}

func ReadFromLatestEvent(subject, eventType string, ifEventIsMissing ifeventismissingduringread.IfEventIsMissing) ReadEventsOption {
	return ReadEventsOption{
		apply: func(options *readEventsOptions) error {
			if options.LowerBoundID != nil {
				return errors.New("ReadFromLowerBoundID and ReadFromLatestEvent are mutually exclusive")
			}
			if err := validateSubject(subject); err != nil {
				return err
			}
			if err := validateEventType(eventType); err != nil {
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
