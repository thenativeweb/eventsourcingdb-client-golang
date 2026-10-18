package eventsourcingdb

import (
	"context"
	"iter"
)

// ObserveEvents reads the events of the given subject, as the given options
// say, and then every event of it that is written from then on. It does not
// end on its own: end it with its context, or with break.
func (c *Client) ObserveEvents(
	ctx context.Context,
	subject string,
	options ObserveEventsOptions,
) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		api, err := c.chooseAPI(ctx, actionObserveEvents)
		if err != nil {
			yield(Event{}, err)
			return
		}

		api.observeEvents(ctx, newObserveEventsRequestBody(subject, options))(yield)
	}
}

// observeEventsRequestBody is the body of a request that observes events,
// which both APIs take alike.
type observeEventsRequestBody struct {
	Subject string                          `json:"subject"`
	Options observeEventsRequestBodyOptions `json:"options"`
}

type observeEventsRequestBodyOptions struct {
	Recursive       bool                                     `json:"recursive"`
	LowerBound      *requestBodyBound                        `json:"lowerBound,omitempty"`
	FromLatestEvent *observeEventsRequestBodyFromLatestEvent `json:"fromLatestEvent,omitempty"`
}

type observeEventsRequestBodyFromLatestEvent struct {
	Subject          string `json:"subject"`
	Type             string `json:"type"`
	IfEventIsMissing string `json:"ifEventIsMissing"`
}

func newObserveEventsRequestBody(subject string, options ObserveEventsOptions) observeEventsRequestBody {
	requestBody := observeEventsRequestBody{
		Subject: subject,
		Options: observeEventsRequestBodyOptions{
			Recursive:  options.Recursive,
			LowerBound: newRequestBodyBound(options.LowerBound),
		},
	}
	if options.FromLatestEvent != nil {
		requestBody.Options.FromLatestEvent = &observeEventsRequestBodyFromLatestEvent{
			Subject:          options.FromLatestEvent.Subject,
			Type:             options.FromLatestEvent.Type,
			IfEventIsMissing: string(options.FromLatestEvent.IfEventIsMissing),
		}
	}

	return requestBody
}
