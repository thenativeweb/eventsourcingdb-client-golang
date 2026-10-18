package eventsourcingdb

import (
	"context"
	"iter"
)

// ReadEventTypes reads every event type that the database knows, together
// with the schema registered for it, if there is one.
func (c *Client) ReadEventTypes(
	ctx context.Context,
) iter.Seq2[EventType, error] {
	return func(yield func(EventType, error) bool) {
		api, err := c.chooseAPI(ctx, actionReadEventTypes)
		if err != nil {
			yield(EventType{}, err)
			return
		}

		api.readEventTypes(ctx)(yield)
	}
}
