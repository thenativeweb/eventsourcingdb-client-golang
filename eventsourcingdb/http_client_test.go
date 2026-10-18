package eventsourcingdb_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An application may change http.DefaultClient for its own requests, e.g. give
// it a Timeout, which would cut every stream of the client short, or another
// transport. The client must talk to the database as before nevertheless.
func TestTheHTTPClient(t *testing.T) {
	t.Run("every request ignores how http.DefaultClient is configured", func(t *testing.T) {
		changeDefaultClient(t)

		client := clientOfAServerThatAnswers(t, http.StatusServiceUnavailable, "service unavailable")

		for _, request := range requestsOf(client) {
			t.Run(request.name, func(t *testing.T) {
				err := request.send()

				// The server answered, so the request did not go through
				// http.DefaultClient, which fails every request.
				assertDBAPIError(t, err, http.StatusServiceUnavailable, "", "service unavailable")
				assert.NotErrorIs(t, err, errDefaultClientUsed)
			})
		}
	})

	t.Run("a stream lasts longer than the Timeout of http.DefaultClient", func(t *testing.T) {
		changeDefaultClient(t)
		http.DefaultClient.Transport = nil
		http.DefaultClient.Timeout = 100 * time.Millisecond

		for _, stream := range streamsWithHeartbeats() {
			t.Run(stream.name, func(t *testing.T) {
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					for range 5 {
						send(heartbeatLine)
						time.Sleep(100 * time.Millisecond)
					}
					send(stream.line)
				})

				items, err := stream.read(context.Background(), client, nil)

				require.NoError(t, err)
				assert.Equal(t, 1, items)
			})
		}
	})
}

var errDefaultClientUsed = errors.New("http.DefaultClient was used")

// changeDefaultClient changes http.DefaultClient until the test t ends, so
// that every request that goes through it fails at once.
func changeDefaultClient(t *testing.T) {
	t.Helper()

	original := *http.DefaultClient
	t.Cleanup(func() {
		*http.DefaultClient = original
	})

	http.DefaultClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errDefaultClientUsed
	})
	http.DefaultClient.Timeout = time.Nanosecond
}

type roundTripperFunc func(request *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
