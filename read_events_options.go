package eventsourcingdb

type ReadEventsOptionsOrder string

const (
	OldestFirst ReadEventsOptionsOrder = "oldest-first"
	NewestFirst ReadEventsOptionsOrder = "newest-first"
)

type ReadEventsOptions struct {
	OptionWithSubStreams *bool                   `json:"withSubStreams,omitempty"`
	OptionOrder          *ReadEventsOptionsOrder `json:"order,omitempty"`
	OptionEventNames     *[]string               `json:"eventNames,omitempty"`
	OptionLowerBoundID   *int                    `json:"lowerBoundId,omitempty"`
	OptionUpperBoundID   *int                    `json:"upperBoundId,omitempty"`
	OptionFromEventName  *string                 `json:"fromEventName,omitempty"`
}

func NewReadEventsOptions() ReadEventsOptions {
	return ReadEventsOptions{}
}

func (options ReadEventsOptions) WithSubStreams(withSubStreams bool) ReadEventsOptions {
	options.OptionWithSubStreams = &withSubStreams

	return options
}

func (options ReadEventsOptions) Order(order ReadEventsOptionsOrder) ReadEventsOptions {
	options.OptionOrder = &order

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
