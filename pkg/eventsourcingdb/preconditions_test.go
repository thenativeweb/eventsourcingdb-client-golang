package eventsourcingdb_test

import (
	"encoding/json"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func removeTabsAndNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(
		text,
		"\t", ""),
		"\n", "")
}

func TestPreconditions(t *testing.T) {
	t.Run("marshals to an empty array.", func(t *testing.T) {
		preconditions := eventsourcingdb.Preconditions()

		jsonResult, err := json.Marshal(preconditions)

		assert.NoError(t, err)
		assert.Equal(t, "[]", string(jsonResult))
	})

	t.Run("marshals all preconditions and flattens them into one array.", func(t *testing.T) {
		preconditions := eventsourcingdb.Preconditions().
			IsSubjectPristine("/bar").
			IsSubjectPristine("/foo").
			IsSubjectOnEventID("/heck", "1337").
			IsSubjectOnEventID("/meck", "420")

		jsonResult, err := json.Marshal(preconditions)

		assert.NoError(t, err)
		assert.Equal(t, removeTabsAndNewlines(`[
			{"type":"isSubjectPristine","payload":{"subject":"/bar"}},
			{"type":"isSubjectPristine","payload":{"subject":"/foo"}},
			{"type":"isSubjectOnEventId","payload":{"subject":"/heck","eventId":"1337"}},
			{"type":"isSubjectOnEventId","payload":{"subject":"/meck","eventId":"420"}}
		]`), string(jsonResult))
	})
}
