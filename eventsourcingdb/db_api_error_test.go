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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
)

// A caller has to be able to tell failures apart that share a status code,
// such as a failed precondition and an event that does not match its schema,
// without parsing the error message.
func TestDBAPIError(t *testing.T) {
	t.Run("every request reports a status other than 200 with its status and description", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusServiceUnavailable, "service unavailable\n")

		for _, request := range requestsOf(client) {
			t.Run(request.name, func(t *testing.T) {
				err := request.send()

				var dbAPIError *eventsourcingdb.DBAPIError
				require.ErrorAs(t, err, &dbAPIError)
				assert.Equal(t, request.action, dbAPIError.Action)
				assert.Equal(t, http.StatusServiceUnavailable, dbAPIError.Status)
				assert.Empty(t, dbAPIError.Code)
				assert.Equal(t, "service unavailable", dbAPIError.Description)
				assert.Nil(t, dbAPIError.Details)
				assert.Zero(t, dbAPIError.RetryAfter)
				assert.NoError(t, dbAPIError.Err)
				assert.EqualError(t, err, fmt.Sprintf("failed to %s, got HTTP status code '503', expected '200': service unavailable", request.action))
			})
		}
	})

	t.Run("every request reports a rejected token with status 401", func(t *testing.T) {
		ctx := context.Background()

		imageTag, err := internal.GetImageTag()
		require.NoError(t, err)

		container := eventsourcingdbtest.NewContainer().WithImageTag(imageTag)
		container.Start(ctx)
		defer container.Stop(ctx)

		baseURL, err := container.GetBaseURL(ctx)
		require.NoError(t, err)

		client, err := eventsourcingdb.NewClient(baseURL, container.GetAPIToken()+"-invalid")
		require.NoError(t, err)

		for _, request := range requestsOf(client) {
			// Ping does not need a token, so it does not fail with an invalid one.
			if request.name == "Ping" {
				continue
			}

			t.Run(request.name, func(t *testing.T) {
				err := request.send()

				assertDBAPIError(t, err, http.StatusUnauthorized, "", "unauthorized")
				assert.EqualError(t, err, fmt.Sprintf("failed to %s, got HTTP status code '401', expected '200': unauthorized", request.action))
			})
		}
	})

	t.Run("every stream reports an error within it with status 200", func(t *testing.T) {
		for _, stream := range streams() {
			t.Run(stream.name, func(t *testing.T) {
				client := clientOfAServerThatStreams(t, func(ctx context.Context, send func(line string)) {
					send(stream.line)
					send(`{"type":"error","payload":{"error":"something failed"}}`)
				})

				items, err := stream.read(context.Background(), client, nil)

				assert.Equal(t, 1, items)
				assertDBAPIError(t, err, http.StatusOK, "", "something failed")
				assert.EqualError(t, err, fmt.Sprintf("failed to %s, got error: something failed", stream.action))
			})
		}
	})

	t.Run("trims white space around the description", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusConflict, "\n  state conflict: precondition failed \n")

		_, err := client.WriteEvents(context.Background(), nil, nil)

		assertDBAPIError(t, err, http.StatusConflict, eventsourcingdb.CodePreconditionViolated, "state conflict: precondition failed")
		assert.EqualError(t, err, "failed to write events, got HTTP status code '409', expected '200': state conflict: precondition failed")
	})

	t.Run("leaves out the description if the server gives none", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusBadGateway, "")

		err := client.Ping(context.Background())

		assertDBAPIError(t, err, http.StatusBadGateway, "", "")
		assert.EqualError(t, err, "failed to ping, got HTTP status code '502', expected '200'")
	})

	t.Run("cuts a long description short", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusBadGateway, strings.Repeat("x", 10_000))

		err := client.Ping(context.Background())

		assertDBAPIError(t, err, http.StatusBadGateway, "", strings.Repeat("x", 4096))
		assert.EqualError(t, err, "failed to ping, got HTTP status code '502', expected '200': "+strings.Repeat("x", 4096))
	})

	t.Run("reports if the description cannot be read", func(t *testing.T) {
		// The server announces a longer description than it sends, so the
		// connection ends before the description does.
		client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "EventSourcingDB/test")
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad gateway"))
		}))

		err := client.Ping(context.Background())

		assertDBAPIError(t, err, http.StatusBadGateway, "", "")
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		assert.EqualError(t, err, "failed to ping, got HTTP status code '502', expected '200', and failed to read the description: unexpected EOF")
	})

	t.Run("reports how long to wait for status 429", func(t *testing.T) {
		retryAfterOf := map[string]time.Duration{
			"3":    3 * time.Second,
			" 1 ":  time.Second,
			"":     0,
			"soon": 0,
			"-1":   0,
		}

		for header, retryAfter := range retryAfterOf {
			t.Run(fmt.Sprintf("Retry-After %q", header), func(t *testing.T) {
				client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Server", "EventSourcingDB/test")
					if header != "" {
						w.Header().Set("Retry-After", header)
					}
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = w.Write([]byte("too many requests"))
				}))

				for _, request := range requestsOf(client) {
					err := request.send()

					var dbAPIError *eventsourcingdb.DBAPIError
					require.ErrorAs(t, err, &dbAPIError, request.name)
					assert.Equal(t, http.StatusTooManyRequests, dbAPIError.Status, request.name)
					assert.Equal(t, retryAfter, dbAPIError.RetryAfter, request.name)
				}
			})
		}
	})

	t.Run("does not report how long to wait for any other status", func(t *testing.T) {
		client := clientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "EventSourcingDB/test")
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusServiceUnavailable)
		}))

		for _, request := range requestsOf(client) {
			err := request.send()

			var dbAPIError *eventsourcingdb.DBAPIError
			require.ErrorAs(t, err, &dbAPIError, request.name)
			assert.Zero(t, dbAPIError.RetryAfter, request.name)
		}
	})

	t.Run("is found if it is wrapped", func(t *testing.T) {
		client := clientOfAServerThatAnswers(t, http.StatusConflict, "state conflict: precondition failed")

		_, err := client.WriteEvents(context.Background(), nil, nil)
		wrapped := fmt.Errorf("failed to handle the command: %w", err)

		assertDBAPIError(t, wrapped, http.StatusConflict, eventsourcingdb.CodePreconditionViolated, "state conflict: precondition failed")
	})

	t.Run("builds its message from its fields", func(t *testing.T) {
		readErr := errors.New("connection reset")

		withDescription := &eventsourcingdb.DBAPIError{Action: "write events", Status: http.StatusConflict, Description: "state conflict: precondition failed"}
		withoutDescription := &eventsourcingdb.DBAPIError{Action: "ping", Status: http.StatusBadGateway}
		withUnreadableDescription := &eventsourcingdb.DBAPIError{Action: "write events", Status: http.StatusConflict, Err: readErr}
		withStatus200 := &eventsourcingdb.DBAPIError{Action: "write events", Status: http.StatusOK, Code: eventsourcingdb.CodePreconditionViolated, Description: "/preconditions/0: precondition isSubjectPristine does not hold"}

		assert.EqualError(t, withDescription, "failed to write events, got HTTP status code '409', expected '200': state conflict: precondition failed")
		assert.EqualError(t, withoutDescription, "failed to ping, got HTTP status code '502', expected '200'")
		assert.EqualError(t, withUnreadableDescription, "failed to write events, got HTTP status code '409', expected '200', and failed to read the description: connection reset")
		assert.ErrorIs(t, withUnreadableDescription, readErr)
		assert.EqualError(t, withStatus200, "failed to write events, got error: /preconditions/0: precondition isSubjectPristine does not hold")
	})
}

// A server with the v1 API reports its errors without a code, and some of them
// as the same status, so the client gives each of those that clearly matches
// an error of the v2 API its code, as EventSourcingDB 1.2 words them.
func TestCodesOfV1Errors(t *testing.T) {
	type v1Error struct {
		request     string
		status      int
		description string
		code        string
	}

	v1Errors := []v1Error{
		{request: "WriteEvents", status: http.StatusConflict, description: "state conflict: precondition failed", code: eventsourcingdb.CodePreconditionViolated},
		{request: "WriteEvents", status: http.StatusConflict, description: "schema conflict: event candidate does not match schema: additionalProperties 'extra' not allowed", code: eventsourcingdb.CodeEventSchemaViolated},
		{request: "RegisterEventSchema", status: http.StatusConflict, description: "schema conflict: schema already exists", code: eventsourcingdb.CodeEventSchemaAlreadyRegistered},
		{request: "ReadEventType", status: http.StatusNotFound, description: "event type 'io.eventsourcingdb.test' not found", code: eventsourcingdb.CodeEventTypeUnknown},
		{request: "WriteEvents", status: http.StatusInsufficientStorage, description: "insufficient storage", code: eventsourcingdb.CodeLicenseInvalid},

		// Every other failure keeps an empty code.
		{request: "WriteEvents", status: http.StatusConflict, description: "conflict", code: ""},
		{request: "WriteEvents", status: http.StatusBadRequest, description: "state conflict: precondition failed", code: ""},
		{request: "RegisterEventSchema", status: http.StatusConflict, description: "schema conflict: event with ID 0: additionalProperties 'extra' not allowed", code: ""},
		{request: "RegisterEventSchema", status: http.StatusInsufficientStorage, description: "insufficient storage", code: ""},
		{request: "ReadEventType", status: http.StatusBadRequest, description: "malformed event type", code: ""},
		{request: "ReadEvents", status: http.StatusNotFound, description: "not found", code: ""},
		{request: "ReadEvents", status: http.StatusConflict, description: "state conflict: fromLatestEvent results in an event ID greater than upperBound ID", code: ""},
	}

	// Every request reports a store in read-only mode with the same text.
	for _, name := range namesOfRequests() {
		v1Errors = append(v1Errors,
			v1Error{request: name, status: http.StatusServiceUnavailable, description: "the current operation mode conflicts with the given request: writing is not permitted at this time", code: eventsourcingdb.CodeStoreReadOnly},
			v1Error{request: name, status: http.StatusServiceUnavailable, description: "service unavailable", code: ""},
		)
	}

	for _, v1Error := range v1Errors {
		t.Run(fmt.Sprintf("%s, %d, %s", v1Error.request, v1Error.status, v1Error.description), func(t *testing.T) {
			client := clientOfAServerThatAnswers(t, v1Error.status, v1Error.description)

			err := requestOf(t, client, v1Error.request).send()

			assertDBAPIError(t, err, v1Error.status, v1Error.code, v1Error.description)
		})
	}
}

// assertDBAPIError asserts that errors.As finds a DBAPIError in err, with the
// given status, code and description.
func assertDBAPIError(t *testing.T, err error, status int, code string, description string) {
	t.Helper()

	var dbAPIError *eventsourcingdb.DBAPIError
	if !assert.ErrorAs(t, err, &dbAPIError) {
		return
	}

	assert.Equal(t, status, dbAPIError.Status)
	assert.Equal(t, code, dbAPIError.Code)
	assert.Equal(t, description, dbAPIError.Description)
}

// assertCodeOfServer asserts that errors.As finds a DBAPIError in err, with
// the given action and code, and with a description that the message of err
// names. It does not assert the wording of the description, which the server
// may change between its versions, nor the status, which is 200 for an error
// that the v2 API reports in the body of its answer.
func assertCodeOfServer(t *testing.T, err error, action string, code string) {
	t.Helper()

	var dbAPIError *eventsourcingdb.DBAPIError
	if !assert.ErrorAs(t, err, &dbAPIError) {
		return
	}

	assert.Equal(t, action, dbAPIError.Action)
	assert.Equal(t, code, dbAPIError.Code)
	assert.NotEmpty(t, dbAPIError.Description)
	assert.NoError(t, dbAPIError.Err)
	assert.ErrorContains(t, err, "failed to "+action+", ")
	assert.ErrorContains(t, err, ": "+dbAPIError.Description)
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

// clientOf returns a client of a server that answers with the given handler,
// configured by the given options.
func clientOf(t *testing.T, handler http.Handler, options ...eventsourcingdb.ClientOption) *eventsourcingdb.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	client, err := eventsourcingdb.NewClient(baseURL, "secret", options...)
	require.NoError(t, err)

	return client
}

type request struct {
	name   string
	action string
	send   func() error
}

// namesOfRequests returns the name of every function of the client that sends
// a request, as requestsOf names them.
func namesOfRequests() []string {
	names := []string{}
	for _, request := range requestsOf(nil) {
		names = append(names, request.name)
	}

	return names
}

// requestOf returns the request of the client with the given name, as
// requestsOf names them.
func requestOf(t *testing.T, client *eventsourcingdb.Client, name string) request {
	t.Helper()

	for _, request := range requestsOf(client) {
		if request.name == name {
			return request
		}
	}

	require.FailNow(t, "there is no request "+name)
	return request{}
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
				for _, err := range client.ReadSubjects(ctx, "/", eventsourcingdb.ReadSubjectsOptions{Recursive: true}) {
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
				_, err := client.ReadEventType(ctx, "io.eventsourcingdb.test")
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
