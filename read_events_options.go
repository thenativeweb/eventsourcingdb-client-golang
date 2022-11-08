package eventsourcingdb

type ReadEventsOptions struct {
	OptionRecursive     bool      `json:"recursive"`
	OptionChronological *bool     `json:"chronological,omitempty"`
	OptionEventNames    *[]string `json:"eventNames,omitempty"`
	OptionLowerBoundID  *int      `json:"lowerBoundId,omitempty"`
	OptionUpperBoundID  *int      `json:"upperBoundId,omitempty"`
	OptionFromEventName *string   `json:"fromEventName,omitempty"`
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

func (options ReadEventsOptions) EventNames(eventNames []string) ReadEventsOptions {
	options.OptionEventNames = &eventNames

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

func (options ReadEventsOptions) FromEventName(eventName string) ReadEventsOptions {
	options.OptionFromEventName = &eventName

	return options
}
