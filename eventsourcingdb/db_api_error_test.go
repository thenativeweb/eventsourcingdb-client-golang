package eventsourcingdb_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

// A caller has to be able to tell failures apart that share a status code,
// such as a failed precondition and an event that does not match its schema,
// without parsing the error message.
func TestDBAPIError(t *testing.T) {
	t.Run("every request reports an unexpected status with its status code and reason", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusServiceUnavailable, "service unavailable\n")

		for _, request := range requestsOf(client) {
			t.Run(request.name, func(t *testing.T) {
				err := request.send()

				var dbAPIError *eventsourcingdb.DBAPIError
				require.ErrorAs(t, err, &dbAPIError)
				assert.Equal(t, request.action, dbAPIError.Action)
				assert.Equal(t, http.StatusServiceUnavailable, dbAPIError.StatusCode)
				assert.Equal(t, "service unavailable", dbAPIError.Reason)
				assert.NoError(t, dbAPIError.Err)
				assert.EqualError(t, err, fmt.Sprintf("failed to %s, got HTTP status code '503', expected '200': service unavailable", request.action))
			})
		}
	})

	t.Run("every request reports a rejected token with status code 401", func(t *testing.T) {
		ctx := context.Background()

		imageVersion, err := internal.GetImageVersionFromDockerfile()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageVersion)
		container.Start(ctx)
		defer container.Stop(ctx)

		baseURL, err := container.GetBaseURL(ctx)
		require.NoError(t, err)

		client, err := eventsourcingdb.NewClient(baseURL, container.GetAPIToken()+"-invalid")
		require.NoError(t, err)

		messages := map[string]string{
			"VerifyAPIToken":      "failed to verify API token, got HTTP status code '401', expected '200': unauthorized",
			"WriteEvents":         "failed to write events, got HTTP status code '401', expected '200': unauthorized",
			"ReadEvents":          "failed to read events, got HTTP status code '401', expected '200': unauthorized",
			"ObserveEvents":       "failed to observe events, got HTTP status code '401', expected '200': unauthorized",
			"RunEventQLQuery":     "failed to run EventQL query, got HTTP status code '401', expected '200': unauthorized",
			"RegisterEventSchema": "failed to register event schema, got HTTP status code '401', expected '200': unauthorized",
			"ReadSubjects":        "failed to read subjects, got HTTP status code '401', expected '200': unauthorized",
			"ReadEventType":       "failed to read event type, got HTTP status code '401', expected '200': unauthorized",
			"ReadEventTypes":      "failed to read event types, got HTTP status code '401', expected '200': unauthorized",
		}

		for _, request := range requestsOf(client) {
			// Ping does not need a token, so it does not fail with an invalid one.
			if request.name == "Ping" {
				continue
			}

			t.Run(request.name, func(t *testing.T) {
				err := request.send()

				assertDBAPIError(t, err, http.StatusUnauthorized, "unauthorized")
				assert.EqualError(t, err, messages[request.name])
			})
		}
	})

	t.Run("trims white space around the reason", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusConflict, "\n  state conflict: precondition failed \n")

		_, err := client.WriteEvents(context.Background(), nil, nil)

		assertDBAPIError(t, err, http.StatusConflict, "state conflict: precondition failed")
		assert.EqualError(t, err, "failed to write events, got HTTP status code '409', expected '200': state conflict: precondition failed")
	})

	t.Run("leaves out the reason if the server gives none", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusBadGateway, "")

		err := client.Ping(context.Background())

		assertDBAPIError(t, err, http.StatusBadGateway, "")
		assert.EqualError(t, err, "failed to ping, got HTTP status code '502', expected '200'")
	})

	t.Run("cuts a long reason short", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusBadGateway, strings.Repeat("x", 10_000))

		err := client.Ping(context.Background())

		assertDBAPIError(t, err, http.StatusBadGateway, strings.Repeat("x", 4096))
		assert.EqualError(t, err, "failed to ping, got HTTP status code '502', expected '200': "+strings.Repeat("x", 4096))
	})

	t.Run("reports if the reason cannot be read", func(t *testing.T) {
		// The server announces a longer reason than it sends, so the
		// connection ends before the reason does.
		client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "EventSourcingDB/test")
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad gateway"))
		}))

		err := client.Ping(context.Background())

		assertDBAPIError(t, err, http.StatusBadGateway, "")
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.EqualError(t, err, "failed to ping, got HTTP status code '502', expected '200', and failed to read the reason: unexpected EOF")
	})

	t.Run("is found if it is wrapped", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusConflict, "state conflict: precondition failed")

		_, err := client.WriteEvents(context.Background(), nil, nil)
		wrapped := fmt.Errorf("failed to handle the command: %w", err)

		assertDBAPIError(t, wrapped, http.StatusConflict, "state conflict: precondition failed")
	})

	t.Run("builds its message from its fields", func(t *testing.T) {
		readErr := errors.New("connection reset")

		withReason := &eventsourcingdb.DBAPIError{Action: "write events", StatusCode: http.StatusConflict, Reason: "state conflict: precondition failed"}
		withoutReason := &eventsourcingdb.DBAPIError{Action: "ping", StatusCode: http.StatusBadGateway}
		withUnreadableReason := &eventsourcingdb.DBAPIError{Action: "write events", StatusCode: http.StatusConflict, Err: readErr}

		assert.EqualError(t, withReason, "failed to write events, got HTTP status code '409', expected '200': state conflict: precondition failed")
		assert.EqualError(t, withoutReason, "failed to ping, got HTTP status code '502', expected '200'")
		assert.EqualError(t, withUnreadableReason, "failed to write events, got HTTP status code '409', expected '200', and failed to read the reason: connection reset")
		assert.ErrorIs(t, withUnreadableReason, readErr)
	})
}

// assertDBAPIError asserts that errors.As finds a DBAPIError in err, with the
// given status code and reason.
func assertDBAPIError(t *testing.T, err error, statusCode int, reason string) {
	t.Helper()

	var dbAPIError *eventsourcingdb.DBAPIError
	if !assert.ErrorAs(t, err, &dbAPIError) {
		return
	}

	assert.Equal(t, statusCode, dbAPIError.StatusCode)
	assert.Equal(t, reason, dbAPIError.Reason)
}

// clientOfAServerThatAnswers returns a client of a server that presents
// itself as EventSourcingDB, and answers every request with the given status
// code and body.
func clientOfAServerThatAnswers(t *testing.T, statusCode int, body string) *eventsourcingdb.Client {
	t.Helper()

	return clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "EventSourcingDB/test")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
}

// clientOf returns a client of a server that answers with the given handler.
func clientOf(t *testing.T, handler http.Handler) *eventsourcingdb.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	client, err := eventsourcingdb.NewClient(baseURL, "secret")
	require.NoError(t, err)

	return client
}

type request struct {
	name   string
	action string
	send   func() error
}

// requestsOf calls every function of the client that sends a request, and
// returns the first error each of them hands out, together with the action it
// names in its errors.
func requestsOf(client *eventsourcingdb.Client) []request {
	ctx := context.Background()

	return []request{
		{
			name:   "Ping",
			action: "ping",
			send: func() error {
				return client.Ping(ctx)
			},
		},
		{
			name:   "VerifyAPIToken",
			action: "verify API token",
			send: func() error {
				return client.VerifyAPIToken(ctx)
			},
		},
		{
			name:   "WriteEvents",
			action: "write events",
			send: func() error {
				_, err := client.WriteEvents(
					ctx,
					[]eventsourcingdb.EventCandidate{
						{
							Source:  "https://www.eventsourcingdb.io",
							Subject: "/test",
							Type:    "io.eventsourcingdb.test",
							Data:    map[string]int{"value": 23},
						},
					},
					nil,
				)
				return err
			},
		},
		{
			name:   "ReadEvents",
			action: "read events",
			send: func() error {
				for _, err := range client.ReadEvents(ctx, "/test", eventsourcingdb.ReadEventsOptions{}) {
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name:   "ObserveEvents",
			action: "observe events",
			send: func() error {
				for _, err := range client.ObserveEvents(ctx, "/test", eventsourcingdb.ObserveEventsOptions{}) {
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name:   "RunEventQLQuery",
			action: "run EventQL query",
			send: func() error {
				for _, err := range client.RunEventQLQuery(ctx, "FROM e IN events PROJECT INTO e") {
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name:   "RegisterEventSchema",
			action: "register event schema",
			send: func() error {
				return client.RegisterEventSchema(ctx, "io.eventsourcingdb.test", map[string]any{"type": "object"})
			},
		},
		{
			name:   "ReadSubjects",
			action: "read subjects",
			send: func() error {
				for _, err := range client.ReadSubjects(ctx, "/") {
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			name:   "ReadEventType",
			action: "read event type",
			send: func() error {
				_, err := client.ReadEventType("io.eventsourcingdb.test")
				return err
			},
		},
		{
			name:   "ReadEventTypes",
			action: "read event types",
			send: func() error {
				for _, err := range client.ReadEventTypes(ctx) {
					if err != nil {
						return err
					}
				}
				return nil
			},
		},
	}
}
