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
	LowerBoundID    *int                 `json:"lowerBoundId,omitempty"`
	UpperBoundID    *int                 `json:"upperBoundId,omitempty"`
	FromLatestEvent *readFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

type ReadEventsOption func(options *readEventsOptions) error

func ReadChronologically() ReadEventsOption {
	return func(options *readEventsOptions) error {
		value := true
		options.Chronological = &value

		return nil
	}
}

func ReadReversedChronologically() ReadEventsOption {
	return func(options *readEventsOptions) error {
		value := false
		options.Chronological = &value

		return nil
	}
}

func ReadFromLowerBoundID(lowerBoundID int) ReadEventsOption {
	return func(options *readEventsOptions) error {
		if options.FromLatestEvent != nil {
			return errors.New("ReadFromLowerBoundID and ReadFromLatestEvent are mutually exclusive")
		}

		options.LowerBoundID = &lowerBoundID

		return nil
	}
}

func ReadUntilUpperBoundID(upperBoundID int) ReadEventsOption {
	return func(options *readEventsOptions) error {
		options.UpperBoundID = &upperBoundID

		return nil
	}
}

func ReadFromLatestEvent(subject, eventType string, ifEventIsMissing IfEventIsMissingDuringRead) ReadEventsOption {
	return func(options *readEventsOptions) error {
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
	}
}
