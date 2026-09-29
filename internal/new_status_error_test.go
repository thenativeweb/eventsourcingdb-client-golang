package internal_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func TestNewStatusError(t *testing.T) {
	t.Run("includes the reason the server gives.", func(t *testing.T) {
		response := &http.Response{
			StatusCode: http.StatusConflict,
			Body:       io.NopCloser(strings.NewReader("state conflict: precondition failed\n")),
		}

		err := internal.NewStatusError("write events", response)

		assert.EqualError(t, err, "failed to write events, got HTTP status code '409', expected '200': state conflict: precondition failed")
	})

	t.Run("leaves out the reason if the server gives none.", func(t *testing.T) {
		response := &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("")),
		}

		err := internal.NewStatusError("ping", response)

		assert.EqualError(t, err, "failed to ping, got HTTP status code '502', expected '200'")
	})

	t.Run("cuts a long reason short.", func(t *testing.T) {
		response := &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 10_000))),
		}

		err := internal.NewStatusError("ping", response)

		assert.Equal(t, "failed to ping, got HTTP status code '502', expected '200': "+strings.Repeat("x", 4096), err.Error())
	})

	t.Run("reports if the reason cannot be read.", func(t *testing.T) {
		readErr := errors.New("connection reset")
		response := &http.Response{
			StatusCode: http.StatusConflict,
			Body:       io.NopCloser(iotest.ErrReader(readErr)),
		}

		err := internal.NewStatusError("write events", response)

		assert.EqualError(t, err, "failed to write events, got HTTP status code '409', expected '200', and failed to read the reason: connection reset")
		assert.ErrorIs(t, err, readErr)
	})
}
