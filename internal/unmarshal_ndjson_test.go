package internal_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestUnmarshalNDJSON(t *testing.T) {
	t.Run("reads multiple lines from the stream.", func(t *testing.T) {
		ctx := context.Background()

		ndjson := `{"type":"first","payload":{"value":23}}` + "\n" +
			`{"type":"second","payload":{"value":42}}` + "\n"

		var lines []internal.Line
		for line, err := range internal.UnmarshalNDJSON(ctx, strings.NewReader(ndjson)) {
			require.NoError(t, err)
			lines = append(lines, line)
		}

		require.Len(t, lines, 2)
		assert.Equal(t, "first", lines[0].Type)
		assert.JSONEq(t, `{"value":23}`, string(lines[0].Payload))
		assert.Equal(t, "second", lines[1].Type)
		assert.JSONEq(t, `{"value":42}`, string(lines[1].Payload))
	})

	t.Run("reads a heavily escaped payload that exceeds the former buffer limit.", func(t *testing.T) {
		ctx := context.Background()

		// Build a payload whose raw size is well beyond the old fixed buffer
		// limit of 100 KB. We use characters that require JSON escaping (a
		// quote and a backslash), so the serialized line is roughly twice the
		// raw size, which is exactly the case the old scanner-based approach
		// could not handle reliably.
		rawValue := strings.Repeat(`"\`, 128*1024)

		payload, err := json.Marshal(map[string]string{
			"value": rawValue,
		})
		require.NoError(t, err)

		ndjson := `{"type":"event","payload":` + string(payload) + "}\n"
		require.Greater(t, len(ndjson), 256*1024)

		var lines []internal.Line
		for line, err := range internal.UnmarshalNDJSON(ctx, strings.NewReader(ndjson)) {
			require.NoError(t, err)
			lines = append(lines, line)
		}

		require.Len(t, lines, 1)
		assert.Equal(t, "event", lines[0].Type)

		var decoded map[string]string
		err = json.Unmarshal(lines[0].Payload, &decoded)
		require.NoError(t, err)

		// Verify the payload is complete and byte-for-byte identical to the
		// value we wrote, not merely that no error occurred.
		assert.Len(t, decoded["value"], len(rawValue))
		assert.Equal(t, rawValue, decoded["value"])
	})

	t.Run("yields an error for a malformed line.", func(t *testing.T) {
		ctx := context.Background()

		ndjson := `{"type":"event","payload":{"value":23}` + "\n"

		var gotError bool
		for _, err := range internal.UnmarshalNDJSON(ctx, strings.NewReader(ndjson)) {
			if err != nil {
				gotError = true
			}
		}

		assert.True(t, gotError)
	})

	t.Run("stops when the context is canceled.", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ndjson := `{"type":"event","payload":{"value":23}}` + "\n"

		var gotError error
		for _, err := range internal.UnmarshalNDJSON(ctx, strings.NewReader(ndjson)) {
			gotError = err
		}

		assert.ErrorIs(t, gotError, context.Canceled)
	})
}
