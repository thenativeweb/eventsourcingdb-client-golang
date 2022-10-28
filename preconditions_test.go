package eventsourcingdb_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thenativeweb/eventsourcingdb-client-golang"
)

func removeTabsAndNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(
		text,
		"\t", ""),
		"\n", "")
}

func TestPreconditions(t *testing.T) {
	t.Run("marshals to an empty array.", func(t *testing.T) {
		preconditions := eventsourcingdb.NewPreconditions()

		jsonResult, err := json.Marshal(preconditions)

		assert.NoError(t, err)
		assert.Equal(t, "[]", string(jsonResult))
	})

	t.Run("marshals all preconditions and flattens them into one array.", func(t *testing.T) {
		preconditions := eventsourcingdb.NewPreconditions().
			IsStreamPristine("/bar").
			IsStreamPristine("/foo").
			IsStreamOnEventID("/heck", 1337).
			IsStreamOnEventID("/meck", 420)

		jsonResult, err := json.Marshal(preconditions)

		assert.NoError(t, err)
		assert.Equal(t, removeTabsAndNewlines(`[
			{"type":"isStreamPristine","payload":{"streamName":"/bar"}},
			{"type":"isStreamPristine","payload":{"streamName":"/foo"}},
			{"type":"isStreamOnEventId","payload":{"streamName":"/heck","eventId":1337}},
			{"type":"isStreamOnEventId","payload":{"streamName":"/meck","eventId":420}}
		]`), string(jsonResult))
	})
}
