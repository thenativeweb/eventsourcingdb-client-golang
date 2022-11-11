package eventsourcingdb

type IfEventIsMissingDuringRead string

const (
	ReadNothingIfEventIsMissingDuringRead    IfEventIsMissingDuringRead = "read-nothing"
	ReadEverythingIfEventIsMissingDuringRead IfEventIsMissingDuringRead = "read-everything"
)

type ReadFromLatestEvent struct {
	StreamName       string                     `json:"streamName"`
	EventName        string                     `json:"eventName"`
	IfEventIsMissing IfEventIsMissingDuringRead `json:"ifEventIsMissing"`
}

type ReadEventsOptions struct {
	OptionRecursive       bool                 `json:"recursive"`
	OptionChronological   *bool                `json:"chronological,omitempty"`
	OptionLowerBoundID    *int                 `json:"lowerBoundId,omitempty"`
	OptionUpperBoundID    *int                 `json:"upperBoundId,omitempty"`
	OptionFromLatestEvent *ReadFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

func NewReadEventsOptions(recursive bool) ReadEventsOptions {
	return ReadEventsOptions{
		OptionRecursive: recursive,
	}
}

func (options ReadEventsOptions) Chronological(chronological bool) ReadEventsOptions {
	options.OptionChronological = &chronological

	return options
}

func (options ReadEventsOptions) LowerBoundID(lowerBoundID int) ReadEventsOptions {
	options.OptionLowerBoundID = &lowerBoundID

	return options
}

func (options ReadEventsOptions) UpperBoundID(upperBoundID int) ReadEventsOptions {
	options.OptionUpperBoundID = &upperBoundID

	return options
}

func (options ReadEventsOptions) FromLatestEvent(fromLatestEvent ReadFromLatestEvent) ReadEventsOptions {
	options.OptionFromLatestEvent = &fromLatestEvent

	return options
}
