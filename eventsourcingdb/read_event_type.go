package eventsourcingdb

import (
	"context"
)

// ReadEventType reads the given event type, together with the schema
// registered for it, if there is one.
func (c *Client) ReadEventType(
	ctx context.Context,
	eventType string,
) (EventType, error) {
	api, err := c.chooseAPI(ctx, actionReadEventType)
	if err != nil {
		return EventType{}, err
	}

	return api.readEventType(ctx, eventType)
}
