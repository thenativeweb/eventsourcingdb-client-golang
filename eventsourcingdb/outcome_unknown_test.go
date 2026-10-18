package eventsourcingdb_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// A caller has to know whether it is safe to write again after a write
// failed. The server never stores a write halfway, but a write whose answer
// is lost may have been stored nonetheless, so the client reports a write as
// not stored only if it could not connect to the server at all, or if the
// server refused it, and every other failure as ErrOutcomeUnknown. The rule
// is the same in every SDK of EventSourcingDB.
func TestErrOutcomeUnknown(t *testing.T) {
	type failure struct {
		name string

		// apis are the APIs the failure applies to.
		apis []apiOfServer

		// write is the only write the failure applies to, if it is not
		// empty.
		write string

		// client returns a client whose write fails as the failure says.
		client func(t *testing.T, api apiOfServer) *eventsourcingdb.Client

		// context returns the context for the write, which it may end.
		context func(t *testing.T) context.Context

		isUnknown bool
		check     func(t *testing.T, err error)
	}

	both := apis()

	answering := func(status int, lines ...string) func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
		return func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
			return clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Server", "EventSourcingDB/test")
				w.WriteHeader(status)
				for _, line := range lines {
					_, _ = w.Write([]byte(line + "\n"))
				}
			}))
		}
	}

	failures := []failure{
		{
			name: "the server can not be connected to",
			apis: both,
			client: func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				address := listener.Addr().String()
				require.NoError(t, listener.Close())

				return clientOfURL(t, api, "http://"+address)
			},
			check: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "connection refused")
			},
		},
		{
			name: "the name of the server can not be resolved",
			apis: both,
			client: func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
				return clientOfURL(t, api, "http://eventsourcingdb.invalid")
			},
			check: func(t *testing.T, err error) {
				var dnsError *net.DNSError
				assert.ErrorAs(t, err, &dnsError)
			},
		},
		{
			name: "the TLS handshake fails",
			apis: both,
			client: func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
				server := httptest.NewTLSServer(http.HandlerFunc(answerAsServer))
				t.Cleanup(server.Close)

				return clientOfURL(t, api, server.URL)
			},
			check: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "certificate")
			},
		},
		{
			name:   "the context ended before the write was sent",
			apis:   both,
			client: answering(http.StatusOK),
			context: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			},
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, context.Canceled)
			},
		},
		{
			name: "the connection stalls once it was established",
			apis: both,
			client: func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
				return clientOfAServerThatHangs(t, api, nil, eventsourcingdb.WithStallTimeout(250*time.Millisecond))
			},
			isUnknown: true,
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, eventsourcingdb.ErrConnectionStalled)
			},
		},
		{
			name: "the connection breaks once the write was sent",
			apis: both,
			client: func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
				return clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					connection, _, err := http.NewResponseController(w).Hijack()
					if err == nil {
						_ = connection.Close()
					}
				}))
			},
			isUnknown: true,
			check: func(t *testing.T, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "the context ends once the write was sent",
			apis: both,
			client: func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
				return clientOfAServerThatHangs(t, api, nil)
			},
			context: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				t.Cleanup(cancel)

				return ctx
			},
			isUnknown: true,
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, context.DeadlineExceeded)
			},
		},
		{
			name: "the server is not EventSourcingDB",
			apis: both,
			client: func(t *testing.T, api apiOfServer) *eventsourcingdb.Client {
				return clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Server", "nginx/1.29.0")
					w.WriteHeader(http.StatusBadGateway)
				}))
			},
			isUnknown: true,
			check: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, eventsourcingdb.ErrInvalidServerHeader)
			},
		},
		{
			name:      "the answer is cut short",
			apis:      []apiOfServer{apiV1},
			write:     "WriteEvents",
			client:    answering(http.StatusOK, `[{"specversion":"1.0",`),
			isUnknown: true,
			check: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "unexpected end of JSON input")
			},
		},
		{
			name:      "the stream breaks off before its end",
			apis:      []apiOfServer{apiV2},
			client:    answering(http.StatusOK, apiV2.heartbeat(), apiV2.line("event", eventOfV2("0"))),
			isUnknown: true,
			check: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "the stream broke off")
			},
		},
		{
			name:      "the server reports that storing failed",
			apis:      []apiOfServer{apiV2},
			client:    answering(http.StatusOK, apiV2.heartbeat(), errorItem(eventsourcingdb.CodeStoreFailed), apiV2.end()),
			isUnknown: true,
			check: func(t *testing.T, err error) {
				assertDBAPIError(t, err, http.StatusOK, eventsourcingdb.CodeStoreFailed, "failed")
			},
		},
	}

	// Every error item but the one that says that storing failed refuses the
	// write.
	for _, code := range []string{eventsourcingdb.CodePreconditionViolated, eventsourcingdb.CodeEventSchemaViolated, eventsourcingdb.CodeStoreReadOnly, eventsourcingdb.CodeServerShuttingDown, eventsourcingdb.CodeLicenseInvalid} {
		failures = append(failures, failure{
			name:   "the server refuses the write with " + code,
			apis:   []apiOfServer{apiV2},
			client: answering(http.StatusOK, apiV2.heartbeat(), errorItem(code), apiV2.end()),
			check: func(t *testing.T, err error) {
				assertDBAPIError(t, err, http.StatusOK, code, "failed")
			},
		})
	}

	statusesOf := []struct {
		api       apiOfServer
		refused   []int
		unknownOf []int
	}{
		{
			api:       apiV1,
			refused:   []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusRequestEntityTooLarge, http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusInsufficientStorage},
			unknownOf: []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout},
		},
		{
			api:       apiV2,
			refused:   []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusRequestTimeout, http.StatusRequestEntityTooLarge, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			unknownOf: []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusNotFound, http.StatusConflict, http.StatusInsufficientStorage},
		},
	}

	for _, statuses := range statusesOf {
		for _, status := range statuses.refused {
			failures = append(failures, failure{
				name:   fmt.Sprintf("the server refuses the write with status %d", status),
				apis:   []apiOfServer{statuses.api},
				client: answering(status, "failed"),
				check: func(t *testing.T, err error) {
					var dbAPIError *eventsourcingdb.DBAPIError
					if assert.ErrorAs(t, err, &dbAPIError) {
						assert.Equal(t, status, dbAPIError.Status)
						assert.Equal(t, "failed", dbAPIError.Description)
					}
				},
			})
		}

		for _, status := range statuses.unknownOf {
			failures = append(failures, failure{
				name:      fmt.Sprintf("the server answers with status %d", status),
				apis:      []apiOfServer{statuses.api},
				client:    answering(status, "failed"),
				isUnknown: true,
				check: func(t *testing.T, err error) {
					var dbAPIError *eventsourcingdb.DBAPIError
					if assert.ErrorAs(t, err, &dbAPIError) {
						assert.Equal(t, status, dbAPIError.Status)
					}
				},
			})
		}
	}

	writes := []struct {
		name string
		what string
		send func(ctx context.Context, client *eventsourcingdb.Client) error
	}{
		{
			name: "WriteEvents",
			what: "the events may have been written",
			send: func(ctx context.Context, client *eventsourcingdb.Client) error {
				_, err := client.WriteEvents(ctx, []eventsourcingdb.EventCandidate{candidate()}, nil)
				return err
			},
		},
		{
			name: "RegisterEventSchema",
			what: "the event schema may have been registered",
			send: func(ctx context.Context, client *eventsourcingdb.Client) error {
				return client.RegisterEventSchema(ctx, "io.eventsourcingdb.test", map[string]any{"type": "object"})
			},
		},
	}

	for _, failure := range failures {
		for _, api := range failure.apis {
			for _, write := range writes {
				if failure.write != "" && failure.write != write.name {
					continue
				}

				t.Run(fmt.Sprintf("%s/%s/%s", api.name, write.name, failure.name), func(t *testing.T) {
					client := failure.client(t, api)

					ctx := context.Background()
					if failure.context != nil {
						ctx = failure.context(t)
					}

					err := write.send(ctx, client)

					require.Error(t, err)
					failure.check(t, err)

					if failure.isUnknown {
						assert.ErrorIs(t, err, eventsourcingdb.ErrOutcomeUnknown)
						assert.True(t, strings.HasPrefix(err.Error(), "outcome unknown, "+write.what+" nevertheless: "), err.Error())
					} else {
						assert.NotErrorIs(t, err, eventsourcingdb.ErrOutcomeUnknown)
					}
				})
			}
		}
	}

	t.Run("a failure to find out which API the server offers leaves the write unsent", func(t *testing.T) {
		client := newClientOf(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "EventSourcingDB/test")
			w.WriteHeader(http.StatusInternalServerError)
		}))

		_, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, nil)

		assertDBAPIError(t, err, http.StatusInternalServerError, "", "")
		assert.NotErrorIs(t, err, eventsourcingdb.ErrOutcomeUnknown)
	})

	t.Run("is found if it is wrapped", func(t *testing.T) {
		client := answering(http.StatusInternalServerError, "failed")(t, apiV2)

		_, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, nil)
		wrapped := fmt.Errorf("failed to handle the command: %w", err)

		assert.ErrorIs(t, wrapped, eventsourcingdb.ErrOutcomeUnknown)

		var dbAPIError *eventsourcingdb.DBAPIError
		require.ErrorAs(t, wrapped, &dbAPIError)
		assert.Equal(t, http.StatusInternalServerError, dbAPIError.Status)
	})

	t.Run("does not apply to any request that does not write", func(t *testing.T) {
		for _, api := range apis() {
			client := clientOfAServerThatAnswers(t, api, http.StatusInternalServerError, "failed")

			for _, request := range requestsOf(client) {
				if request.name == "WriteEvents" || request.name == "RegisterEventSchema" {
					continue
				}

				err := request.send()

				assertDBAPIError(t, err, http.StatusInternalServerError, "", "failed")
				assert.NotErrorIs(t, err, eventsourcingdb.ErrOutcomeUnknown, request.name)
			}
		}
	})
}

// clientOfURL returns a client of the server at the given URL, which speaks
// the given API, as if the client had asked the server already.
func clientOfURL(t *testing.T, api apiOfServer, rawURL string) *eventsourcingdb.Client {
	t.Helper()

	baseURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	client, err := eventsourcingdb.NewClient(baseURL, "secret")
	require.NoError(t, err)
	eventsourcingdb.UseAPI(client, api.name)

	return client
}

// errorItem returns an error item of the v2 API with the given code and the
// description "failed".
func errorItem(code string) string {
	return apiV2.line("error", `{"code":"`+code+`","description":"failed","details":{}}`)
}
