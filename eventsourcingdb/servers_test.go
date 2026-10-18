package eventsourcingdb_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// apiOfServer is an API that a fake server speaks, so that a test runs
// against a server with the v1 API as well as against one with the v2 API.
type apiOfServer struct {
	name string

	// heartbeats tells for a stream whether the server sends heartbeats on it,
	// by the name of the function of the client that reads it.
	heartbeats func(name string) bool
}

var (
	apiV1 = apiOfServer{
		name: "v1",
		heartbeats: func(name string) bool {
			return name == "ObserveEvents" || name == "RunEventQLQuery"
		},
	}
	apiV2 = apiOfServer{
		name: "v2",
		heartbeats: func(string) bool {
			return true
		},
	}
)

// apis returns every API that a fake server speaks.
func apis() []apiOfServer {
	return []apiOfServer{apiV1, apiV2}
}

// line returns a line of a stream of the API with the given type and payload,
// e.g. {"type":"event","payload":{...}} for the v1 API, and
// {"type":"event","data":{...}} for the v2 API.
func (api apiOfServer) line(lineType string, payload string) string {
	if api.name == "v1" {
		return `{"type":"` + lineType + `","payload":` + payload + `}`
	}

	return `{"type":"` + lineType + `","data":` + payload + `}`
}

// heartbeat returns the line of a heartbeat of the API.
func (api apiOfServer) heartbeat() string {
	return api.line("heartbeat", `{}`)
}

// end returns the line that ends a finite stream of the API, which the v1 API
// does not have.
func (api apiOfServer) end() string {
	if api.name == "v1" {
		return ""
	}

	return api.line("end", `{}`)
}

// pingAnswer returns the body with which a server of the API answers a ping.
func (api apiOfServer) pingAnswer() string {
	event := `{"specversion":"1.0","id":"0","time":"2026-10-01T12:00:00Z","source":"https://www.eventsourcingdb.io","subject":"/api/v2/system/ping","type":"io.eventsourcingdb.api.ping-received","datacontenttype":"application/json","data":{"message":"My God, it's full of stars."}}`
	if api.name == "v1" {
		return event
	}

	return `{"type":"event","data":` + event + `}`
}

// clientOf returns a client of a server that answers with the given handler,
// configured by the given options, and that speaks the given API, as if the
// client had asked the server already.
func clientOf(t *testing.T, api apiOfServer, handler http.Handler, options ...eventsourcingdb.ClientOption) *eventsourcingdb.Client {
	t.Helper()

	client := newClientOf(t, handler, options...)
	eventsourcingdb.UseAPI(client, api.name)

	return client
}

// newClientOf returns a client of a server that answers with the given
// handler, configured by the given options, that has not asked the server for
// its API yet.
func newClientOf(t *testing.T, handler http.Handler, options ...eventsourcingdb.ClientOption) *eventsourcingdb.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	client, err := eventsourcingdb.NewClient(baseURL, "secret", options...)
	require.NoError(t, err)

	return client
}

// answeringTheProbe returns a handler that answers the request with which a
// client asks a server for its API as a server of the given API does, and
// hands every other request to handler. As the v2 API answers that request
// with a ping, it hands every ping after the first one to handler.
func answeringTheProbe(api apiOfServer, handler http.Handler) http.Handler {
	var isAnswered atomic.Bool

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/system/ping" || (api.name == "v2" && isAnswered.Swap(true)) {
			handler.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Server", "EventSourcingDB/test")
		if api.name == "v1" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(api.pingAnswer()))
	})
}

// clientOfAServerThatAnswers returns a client of a server that presents
// itself as EventSourcingDB, speaks the given API, and answers every request
// with the given status code and body.
func clientOfAServerThatAnswers(t *testing.T, api apiOfServer, statusCode int, body string) *eventsourcingdb.Client {
	t.Helper()

	return clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "EventSourcingDB/test")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
}

// clientOfAServerThatStreams returns a client of a server that answers every
// request as EventSourcingDB with the given API would, and then sends every
// line that stream hands to send, right away. Once stream returns, the server
// ends the response, with the line that ends a finite stream of the API, if it
// has one. The context that stream gets ends when the client closes the
// connection, and every stream that waits for it also has to give up after a
// few seconds, so that a client that does not close the connection still
// ends. The client is configured by the given options.
func clientOfAServerThatStreams(t *testing.T, api apiOfServer, stream func(ctx context.Context, send func(line string)), options ...eventsourcingdb.ClientOption) *eventsourcingdb.Client {
	t.Helper()

	return clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "EventSourcingDB/test")
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()

		send := func(line string) {
			_, _ = w.Write([]byte(line + "\n"))
			w.(http.Flusher).Flush()
		}

		stream(r.Context(), send)

		if api.end() != "" && !strings.HasSuffix(r.URL.Path, "/observe-events") {
			send(api.end())
		}
	}), options...)
}

// clientOfAServerThatHangs returns a client of a server that answers as
// EventSourcingDB with the given API would, but stops: without a start of a
// stream, before it answers at all, and with one, after sending it. It waits
// until the request ends, until the test is over, or until five seconds have
// passed, so that a client that ignores its context still ends. The client is
// configured by the given options.
func clientOfAServerThatHangs(t *testing.T, api apiOfServer, start []byte, options ...eventsourcingdb.ClientOption) *eventsourcingdb.Client {
	t.Helper()

	// The server waits for its handlers when it closes, so they have to be
	// released first, and cleanups run in the reverse order.
	released := make(chan struct{})
	client := clientOf(t, api, hangingHandler(start, released), options...)
	t.Cleanup(func() {
		close(released)
	})

	return client
}

// hangingHandler answers as clientOfAServerThatHangs describes, until
// released is closed.
func hangingHandler(start []byte, released <-chan struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if start != nil {
			w.Header().Set("Server", "EventSourcingDB/test")
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(start)
			w.(http.Flusher).Flush()
		}

		select {
		case <-r.Context().Done():
		case <-released:
		case <-time.After(5 * time.Second):
		}
	}
}

type stream struct {
	name    string
	action  string
	endless bool

	// item returns a line of the given API that carries one item of the
	// stream.
	item func(api apiOfServer) string

	// read reads until the stream ends, calls handle for every item if it is
	// given, and returns how many items there were and the error the stream
	// ends with.
	read func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error)
}

// streams returns every function of the client that reads a stream.
func streams() []stream {
	event := `{"specversion":"1.0","id":"0","time":"2026-10-01T12:00:00Z","source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","datacontenttype":"application/json","data":{},"hash":"","predecessorhash":""}`

	return []stream{
		{
			name:   "ReadEvents",
			action: "read events",
			item: func(api apiOfServer) string {
				return api.line("event", event)
			},
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				return countItems(client.ReadEvents(ctx, "/test", eventsourcingdb.ReadEventsOptions{}), handle)
			},
		},
		{
			name:    "ObserveEvents",
			action:  "observe events",
			endless: true,
			item: func(api apiOfServer) string {
				return api.line("event", event)
			},
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				return countItems(client.ObserveEvents(ctx, "/test", eventsourcingdb.ObserveEventsOptions{}), handle)
			},
		},
		{
			name:   "RunEventQLQuery",
			action: "run EventQL query",
			item: func(api apiOfServer) string {
				return api.line("row", `{"value":23}`)
			},
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				return countItems(client.RunEventQLQuery(ctx, "FROM e IN events PROJECT INTO e"), handle)
			},
		},
		{
			name:   "ReadSubjects",
			action: "read subjects",
			item: func(api apiOfServer) string {
				return api.line("subject", `{"subject":"/test"}`)
			},
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				return countItems(client.ReadSubjects(ctx, "/", eventsourcingdb.ReadSubjectsOptions{Recursive: true}), handle)
			},
		},
		{
			name:   "ReadEventTypes",
			action: "read event types",
			item: func(api apiOfServer) string {
				if api.name == "v1" {
					return api.line("eventType", `{"eventType":"io.eventsourcingdb.test","isPhantom":false}`)
				}

				return api.line("eventType", `{"type":"io.eventsourcingdb.test","hasEvents":true,"schema":null}`)
			},
			read: func(ctx context.Context, client *eventsourcingdb.Client, handle func()) (int, error) {
				return countItems(client.ReadEventTypes(ctx), handle)
			},
		},
	}
}

// streamsWithHeartbeats returns every function of the client that reads a
// stream on which a server of the given API sends heartbeats.
func streamsWithHeartbeats(api apiOfServer) []stream {
	withHeartbeats := []stream{}
	for _, stream := range streams() {
		if api.heartbeats(stream.name) {
			withHeartbeats = append(withHeartbeats, stream)
		}
	}

	return withHeartbeats
}

// countItems reads the items of a stream until it ends, calls handle for
// every item if it is given, and returns how many items there were and the
// error the stream ends with.
func countItems[T any](items func(yield func(T, error) bool), handle func()) (int, error) {
	count := 0
	for _, err := range items {
		if err != nil {
			return count, err
		}

		count++
		if handle != nil {
			handle()
		}
	}

	return count, nil
}

// assertEndedCleanly asserts that a stream that the server ended on its own
// ended as it should: without an error, except for a stream of the v2 API
// that never ends, which reports that it broke off.
func assertEndedCleanly(t *testing.T, api apiOfServer, stream stream, err error) {
	t.Helper()

	if api.name == "v2" && stream.endless {
		assert.EqualError(t, err, fmt.Sprintf("failed to %s, the stream broke off", stream.action))
		return
	}

	assert.NoError(t, err)
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

// requestsWithContextOf calls every function of the client that takes a
// context, and returns the first error each of them hands out.
func requestsWithContextOf(client *eventsourcingdb.Client) map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"Ping": func(ctx context.Context) error {
			return client.Ping(ctx)
		},
		"VerifyAPIToken": func(ctx context.Context) error {
			return client.VerifyAPIToken(ctx)
		},
		"WriteEvents": func(ctx context.Context) error {
			_, err := client.WriteEvents(ctx, []eventsourcingdb.EventCandidate{
				{
					Source:  "https://www.eventsourcingdb.io",
					Subject: "/test",
					Type:    "io.eventsourcingdb.test",
					Data:    map[string]int{"value": 23},
				},
			}, nil)
			return err
		},
		"RegisterEventSchema": func(ctx context.Context) error {
			return client.RegisterEventSchema(ctx, "io.eventsourcingdb.test", map[string]any{"type": "object"})
		},
		"ReadEventType": func(ctx context.Context) error {
			_, err := client.ReadEventType(ctx, "io.eventsourcingdb.test")
			return err
		},
		"ReadEvents": func(ctx context.Context) error {
			for _, err := range client.ReadEvents(ctx, "/test", eventsourcingdb.ReadEventsOptions{}) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"ObserveEvents": func(ctx context.Context) error {
			for _, err := range client.ObserveEvents(ctx, "/test", eventsourcingdb.ObserveEventsOptions{}) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"ReadSubjects": func(ctx context.Context) error {
			for _, err := range client.ReadSubjects(ctx, "/", eventsourcingdb.ReadSubjectsOptions{Recursive: true}) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"ReadEventTypes": func(ctx context.Context) error {
			for _, err := range client.ReadEventTypes(ctx) {
				if err != nil {
					return err
				}
			}
			return nil
		},
		"RunEventQLQuery": func(ctx context.Context) error {
			for _, err := range client.RunEventQLQuery(ctx, "FROM e IN events PROJECT INTO e") {
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}
