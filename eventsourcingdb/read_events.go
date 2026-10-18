package eventsourcingdb

import (
	"context"
	"iter"
)

// ReadEvents reads the events of the given subject, as the given options
// say, and ends once it has read all of them.
func (c *Client) ReadEvents(
	ctx context.Context,
	subject string,
	options ReadEventsOptions,
) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		api, err := c.chooseAPI(ctx, actionReadEvents)
		if err != nil {
			yield(Event{}, err)
			return
		}

		api.readEvents(ctx, newReadEventsRequestBody(subject, options))(yield)
	}
}

// readEventsRequestBody is the body of a request that reads events, which
// both APIs take alike.
type readEventsRequestBody struct {
	Subject string                       `json:"subject"`
	Options readEventsRequestBodyOptions `json:"options"`
}

type readEventsRequestBodyOptions struct {
	Recursive       bool                                  `json:"recursive"`
	Order           *string                               `json:"order,omitempty"`
	LowerBound      *requestBodyBound                     `json:"lowerBound,omitempty"`
	UpperBound      *requestBodyBound                     `json:"upperBound,omitempty"`
	FromLatestEvent *readEventsRequestBodyFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

type readEventsRequestBodyFromLatestEvent struct {
	Subject          string `json:"subject"`
	Type             string `json:"type"`
	IfEventIsMissing string `json:"ifEventIsMissing"`
}

type requestBodyBound struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func newRequestBodyBound(bound *Bound) *requestBodyBound {
	if bound == nil {
		return nil
	}

	return &requestBodyBound{
		ID:   bound.ID,
		Type: string(bound.Type),
	}
}

func newReadEventsRequestBody(subject string, options ReadEventsOptions) readEventsRequestBody {
	requestBody := readEventsRequestBody{
		Subject: subject,
		Options: readEventsRequestBodyOptions{
			Recursive:  options.Recursive,
			LowerBound: newRequestBodyBound(options.LowerBound),
			UpperBound: newRequestBodyBound(options.UpperBound),
		},
	}
	if options.Order != nil {
		order := string(*options.Order)
		requestBody.Options.Order = &order
	}
	if options.FromLatestEvent != nil {
		requestBody.Options.FromLatestEvent = &readEventsRequestBodyFromLatestEvent{
			Subject:          options.FromLatestEvent.Subject,
			Type:             options.FromLatestEvent.Type,
			IfEventIsMissing: string(options.FromLatestEvent.IfEventIsMissing),
		}
	}

	return requestBody
}
