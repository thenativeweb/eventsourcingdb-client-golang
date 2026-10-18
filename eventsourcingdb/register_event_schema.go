package eventsourcingdb

import (
	"context"
)

// RegisterEventSchema registers the given schema for the given event type,
// which every event of that type has to match from then on.
func (c *Client) RegisterEventSchema(ctx context.Context, eventType string, schema map[string]any) error {
	api, err := c.chooseAPI(ctx, actionRegisterEventSchema)
	if err != nil {
		return err
	}

	return api.registerEventSchema(ctx, eventType, schema)
}
