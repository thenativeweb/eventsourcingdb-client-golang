package eventsourcingdb

import (
	"context"
	"encoding/json"
	"iter"
)

// RunEventQLQuery runs the given EventQL query, and reads the rows it
// returns, each as the JSON of what the query projects into.
func (c *Client) RunEventQLQuery(
	ctx context.Context,
	query string,
) iter.Seq2[json.RawMessage, error] {
	return func(yield func(json.RawMessage, error) bool) {
		api, err := c.chooseAPI(ctx, actionRunEventQLQuery)
		if err != nil {
			yield(nil, err)
			return
		}

		api.runEventQLQuery(ctx, query)(yield)
	}
}
