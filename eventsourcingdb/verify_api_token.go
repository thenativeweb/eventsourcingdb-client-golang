package eventsourcingdb

import (
	"context"
)

// VerifyAPIToken checks whether the server accepts the API token of the
// client.
func (c *Client) VerifyAPIToken(ctx context.Context) error {
	api, err := c.chooseAPI(ctx, actionVerifyAPIToken)
	if err != nil {
		return err
	}

	return api.verifyAPIToken(ctx)
}
