package eventsourcingdb

import (
	"context"
	"iter"
)

// ReadSubjects reads the given subject, and, if the Recursive option is set,
// every subject nested below it.
func (c *Client) ReadSubjects(
	ctx context.Context,
	subject string,
	options ReadSubjectsOptions,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		api, err := c.chooseAPI(ctx, actionReadSubjects)
		if err != nil {
			yield("", err)
			return
		}

		api.readSubjects(ctx, subject, options)(yield)
	}
}
