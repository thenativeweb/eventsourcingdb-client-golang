package eventsourcingdb

import (
	"context"
)

// typeOfPingReceived is the type of the event with which the server answers a
// ping.
const typeOfPingReceived = "io.eventsourcingdb.api.ping-received"

// Ping checks whether the server is reachable. It does not need a valid API
// token.
func (c *Client) Ping(ctx context.Context) error {
	api, err := c.chooseAPI(ctx, actionPing)
	if err != nil {
		return err
	}

	return api.ping(ctx)
}
