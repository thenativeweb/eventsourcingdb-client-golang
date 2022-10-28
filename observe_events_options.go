package eventsourcingdb

type ObserveEventsOptions struct {
	OptionWithSubStreams *bool     `json:"withSubStreams,omitempty"`
	OptionEventNames     *[]string `json:"eventNames,omitempty"`
	OptionLowerBoundID   *int64    `json:"lowerBoundId,omitempty"`
	OptionFromEventName  *string   `json:"fromEventName,omitempty"`
}

func NewObserveEventsOptions() ObserveEventsOptions {
	return ObserveEventsOptions{}
}

func (options ObserveEventsOptions) WithSubStreams(withSubStreams bool) ObserveEventsOptions {
	options.OptionWithSubStreams = &withSubStreams

	return options
}

func (options ObserveEventsOptions) EventNames(eventNames []string) ObserveEventsOptions {
	options.OptionEventNames = &eventNames

	return options
}

func (options ObserveEventsOptions) LowerBoundID(lowerBoundID int64) ObserveEventsOptions {
	options.OptionLowerBoundID = &lowerBoundID

	return options
}

func (options ObserveEventsOptions) FromEventName(eventName string) ObserveEventsOptions {
	options.OptionFromEventName = &eventName

	return options
}
