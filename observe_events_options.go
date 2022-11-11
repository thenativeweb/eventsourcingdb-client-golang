package eventsourcingdb

type ObserveEventsOptions struct {
	OptionRecursive     bool    `json:"recursive"`
	OptionLowerBoundID  *int    `json:"lowerBoundId,omitempty"`
	OptionFromEventName *string `json:"fromEventName,omitempty"`
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

func (options ObserveEventsOptions) FromEventName(eventName string) ObserveEventsOptions {
	options.OptionFromEventName = &eventName

	return options
}
