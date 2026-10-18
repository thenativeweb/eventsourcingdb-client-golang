package eventsourcingdb_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// A server with the v2 API takes every request in the shape that its v2 API
// documents, and refuses fields it does not know, and null, while a server
// with the v1 API only takes the requests of the v1 API. The client has to
// send each request as the API it speaks expects it, and read the answer as
// that API sends it.
func TestTheAPIs(t *testing.T) {
	type sentRequest struct {
		method string
		path   string
		body   string
	}

	sentRequests := map[string]map[string]sentRequest{
		"v1": {
			"Ping":                {method: "GET", path: "/api/v1/ping"},
			"VerifyAPIToken":      {method: "POST", path: "/api/v1/verify-api-token"},
			"WriteEvents":         {method: "POST", path: "/api/v1/write-events", body: `{"events":[{"source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","data":{"value":23}}]}`},
			"RegisterEventSchema": {method: "POST", path: "/api/v1/register-event-schema", body: `{"eventType":"io.eventsourcingdb.test","schema":{"type":"object"}}`},
			"ReadEventType":       {method: "POST", path: "/api/v1/read-event-type", body: `{"eventType":"io.eventsourcingdb.test"}`},
			"ReadEvents":          {method: "POST", path: "/api/v1/read-events", body: `{"subject":"/test","options":{"recursive":false}}`},
			"ObserveEvents":       {method: "POST", path: "/api/v1/observe-events", body: `{"subject":"/test","options":{"recursive":false}}`},
			"RunEventQLQuery":     {method: "POST", path: "/api/v1/run-eventql-query", body: `{"query":"FROM e IN events PROJECT INTO e"}`},
			"ReadSubjects":        {method: "POST", path: "/api/v1/read-subjects", body: `{"baseSubject":"/"}`},
			"ReadEventTypes":      {method: "POST", path: "/api/v1/read-event-types", body: `{}`},
		},
		"v2": {
			"Ping":                {method: "GET", path: "/api/v2/system/ping"},
			"VerifyAPIToken":      {method: "POST", path: "/api/v2/system/verify-token"},
			"WriteEvents":         {method: "POST", path: "/api/v2/data/write-events", body: `{"events":[{"source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","data":{"value":23}}]}`},
			"RegisterEventSchema": {method: "POST", path: "/api/v2/data/register-event-schema", body: `{"type":"io.eventsourcingdb.test","schema":{"type":"object"}}`},
			"ReadEventType":       {method: "QUERY", path: "/api/v2/data/read-event-type", body: `{"type":"io.eventsourcingdb.test"}`},
			"ReadEvents":          {method: "QUERY", path: "/api/v2/data/read-events", body: `{"subject":"/test","options":{"recursive":false}}`},
			"ObserveEvents":       {method: "QUERY", path: "/api/v2/data/observe-events", body: `{"subject":"/test","options":{"recursive":false}}`},
			"RunEventQLQuery":     {method: "QUERY", path: "/api/v2/data/run-eventql-query", body: `{"query":"FROM e IN events PROJECT INTO e"}`},
			"ReadSubjects":        {method: "QUERY", path: "/api/v2/data/read-subjects", body: `{"subject":"/","options":{"recursive":true}}`},
			"ReadEventTypes":      {method: "QUERY", path: "/api/v2/data/read-event-types"},
		},
	}

	for _, api := range apis() {
		t.Run(api.name, func(t *testing.T) {
			t.Run("sends every request as the API expects it", func(t *testing.T) {
				var mu sync.Mutex
				var received []*http.Request
				var bodies []string

				client := clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)

					mu.Lock()
					received = append(received, r)
					bodies = append(bodies, string(body))
					mu.Unlock()

					answerAsServer(w, r)
				}))

				for _, request := range requestsOf(client) {
					t.Run(request.name, func(t *testing.T) {
						mu.Lock()
						before := len(received)
						mu.Unlock()

						err := request.send()

						// Observing never ends, so a server with the v2 API
						// that ends it fails it.
						if api.name == "v2" && request.name == "ObserveEvents" {
							assert.EqualError(t, err, "failed to observe events, the stream broke off")
						} else {
							require.NoError(t, err)
						}

						mu.Lock()
						defer mu.Unlock()
						require.Len(t, received, before+1)

						expected := sentRequests[api.name][request.name]
						sent := received[before]
						assert.Equal(t, expected.method, sent.Method)
						assert.Equal(t, expected.path, sent.URL.Path)

						if expected.body == "" {
							assert.Empty(t, bodies[before])
							assert.Empty(t, sent.Header.Get("Content-Type"))
						} else {
							assert.JSONEq(t, expected.body, bodies[before])
							assert.Equal(t, "application/json", sent.Header.Get("Content-Type"))
						}

						if request.name == "Ping" {
							assert.Empty(t, sent.Header.Get("Authorization"))
						} else {
							assert.Equal(t, "Bearer secret", sent.Header.Get("Authorization"))
						}
					})
				}
			})

			t.Run("sends what an event leaves out by leaving it out, rather than as null", func(t *testing.T) {
				var body string
				client := clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					bodyBytes, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					body = string(bodyBytes)

					answerAsServer(w, r)
				}))

				_, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, nil)
				require.NoError(t, err)

				assert.NotContains(t, body, "null")
				assert.NotContains(t, body, "traceparent")
				assert.NotContains(t, body, "tracestate")
				assert.NotContains(t, body, "preconditions")
			})

			t.Run("sends an empty list of events as such, rather than as null", func(t *testing.T) {
				var body string
				client := clientOf(t, api, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					bodyBytes, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					body = string(bodyBytes)

					answerAsServer(w, r)
				}))

				_, _ = client.WriteEvents(context.Background(), nil, nil)

				assert.JSONEq(t, `{"events":[]}`, body)
			})
		})
	}

	t.Run("v2", func(t *testing.T) {
		t.Run("reads the events that a write returns", func(t *testing.T) {
			client := clientOfAServerThatStreams(t, apiV2, func(ctx context.Context, send func(line string)) {
				send(apiV2.heartbeat())
				send(apiV2.line("event", eventOfV2("0")))
				send(apiV2.heartbeat())
				send(apiV2.line("event", eventOfV2("1")))
			})

			events, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate(), candidate()}, nil)

			require.NoError(t, err)
			require.Len(t, events, 2)
			assert.Equal(t, "0", events[0].ID)
			assert.Equal(t, "1", events[1].ID)
			assert.Equal(t, "/test", events[0].Subject)
			assert.JSONEq(t, `{"value":23}`, string(events[0].Data))
			assert.Nil(t, events[0].TraceParent)
			assert.Nil(t, events[0].TraceState)
			assert.Nil(t, events[0].Signature)
		})

		t.Run("fails to register an event schema if the stream ends without the event that confirms it", func(t *testing.T) {
			client := clientOfAServerThatStreams(t, apiV2, func(ctx context.Context, send func(line string)) {
				send(apiV2.heartbeat())
			})

			err := client.RegisterEventSchema(context.Background(), "io.eventsourcingdb.test", map[string]any{"type": "object"})

			assertErrorOfRequest(t, "RegisterEventSchema", err, "failed to register event schema, the stream ended without the event that confirms it")
		})

		t.Run("reads an event type with and without a schema", func(t *testing.T) {
			answers := map[string]eventsourcingdb.EventType{
				`{"type":"io.eventsourcingdb.test","hasEvents":true,"schema":null}`: {
					Type:      "io.eventsourcingdb.test",
					HasEvents: true,
				},
				`{"type":"io.eventsourcingdb.test","hasEvents":false,"schema":{"type":"object"}}`: {
					Type:      "io.eventsourcingdb.test",
					HasEvents: false,
					Schema:    &map[string]any{"type": "object"},
				},
			}

			for answer, expected := range answers {
				client := clientOfAServerThatAnswers(t, apiV2, http.StatusOK, apiV2.line("eventType", answer))

				eventType, err := client.ReadEventType(context.Background(), "io.eventsourcingdb.test")

				require.NoError(t, err)
				assert.Equal(t, expected, eventType)

				streamingClient := clientOfAServerThatStreams(t, apiV2, func(ctx context.Context, send func(line string)) {
					send(apiV2.line("eventType", answer))
				})

				eventTypes := []eventsourcingdb.EventType{}
				for eventType, err := range streamingClient.ReadEventTypes(context.Background()) {
					require.NoError(t, err)
					eventTypes = append(eventTypes, eventType)
				}
				assert.Equal(t, []eventsourcingdb.EventType{expected}, eventTypes)
			}
		})

		t.Run("hands out what a query projects into, without the row that wraps it", func(t *testing.T) {
			values := []string{`{"id":"0"}`, `23`, `"text"`, `null`}

			client := clientOfAServerThatStreams(t, apiV2, func(ctx context.Context, send func(line string)) {
				for _, value := range values {
					send(apiV2.line("row", `{"value":`+value+`}`))
				}
			})

			rows := []string{}
			for row, err := range client.RunEventQLQuery(context.Background(), "FROM e IN events PROJECT INTO e") {
				require.NoError(t, err)
				rows = append(rows, string(row))
			}

			assert.Equal(t, values, rows)
		})

		t.Run("every finite stream fails if it breaks off before its end", func(t *testing.T) {
			finite := []struct {
				name   string
				action string
				item   string
				read   func(client *eventsourcingdb.Client) error
			}{
				{name: "WriteEvents", action: "write events", item: apiV2.line("event", eventOfV2("0")), read: func(client *eventsourcingdb.Client) error {
					_, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, nil)
					return err
				}},
				{name: "RegisterEventSchema", action: "register event schema", item: apiV2.line("event", `{"type":"io.eventsourcingdb.api.event-schema-registered"}`), read: func(client *eventsourcingdb.Client) error {
					return client.RegisterEventSchema(context.Background(), "io.eventsourcingdb.test", map[string]any{"type": "object"})
				}},
			}
			for _, stream := range streams() {
				if stream.endless {
					continue
				}

				finite = append(finite, struct {
					name   string
					action string
					item   string
					read   func(client *eventsourcingdb.Client) error
				}{name: stream.name, action: stream.action, item: stream.item(apiV2), read: func(client *eventsourcingdb.Client) error {
					_, err := stream.read(context.Background(), client, nil)
					return err
				}})
			}

			for _, stream := range finite {
				for _, lines := range [][]string{{}, {apiV2.heartbeat()}, {stream.item}, {stream.item, apiV2.heartbeat()}} {
					t.Run(fmt.Sprintf("%s after %d lines", stream.name, len(lines)), func(t *testing.T) {
						client := clientOf(t, apiV2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("Server", "EventSourcingDB/test")
							w.WriteHeader(http.StatusOK)
							for _, line := range lines {
								_, _ = w.Write([]byte(line + "\n"))
							}
						}))

						err := stream.read(client)

						assert.ErrorContains(t, err, fmt.Sprintf("failed to %s, the stream broke off before its end", stream.action))
					})
				}
			}
		})

		t.Run("observing fails if the stream ends", func(t *testing.T) {
			for _, end := range []string{"", apiV2.end()} {
				client := clientOf(t, apiV2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Server", "EventSourcingDB/test")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(apiV2.line("event", eventOfV2("0")) + "\n" + end))
				}))

				items, err := countItems(client.ObserveEvents(context.Background(), "/test", eventsourcingdb.ObserveEventsOptions{}), nil)

				assert.Equal(t, 1, items)
				assert.Error(t, err)
			}
		})

		t.Run("takes no code from the description of an error, as the v1 API needs", func(t *testing.T) {
			client := clientOfAServerThatAnswers(t, apiV2, http.StatusConflict, "state conflict: precondition failed")

			_, err := client.WriteEvents(context.Background(), []eventsourcingdb.EventCandidate{candidate()}, nil)

			assertDBAPIError(t, err, http.StatusConflict, "", "state conflict: precondition failed")
		})
	})

	t.Run("v1", func(t *testing.T) {
		t.Run("reads the given subject alone by keeping only the subject itself", func(t *testing.T) {
			var bodies []string
			client := clientOf(t, apiV1, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				bodies = append(bodies, string(body))

				w.Header().Set("Server", "EventSourcingDB/test")
				for _, subject := range []string{"/test", "/test/1", "/test/1/a", "/test/2"} {
					_, _ = w.Write([]byte(apiV1.line("subject", `{"subject":"`+subject+`"}`) + "\n"))
				}
			}))

			subjectsOf := []struct {
				subject   string
				recursive bool
				subjects  []string
			}{
				{subject: "/test", recursive: true, subjects: []string{"/test", "/test/1", "/test/1/a", "/test/2"}},
				{subject: "/test", recursive: false, subjects: []string{"/test"}},
				{subject: "/test/1", recursive: false, subjects: []string{"/test/1"}},
				{subject: "/test/3", recursive: false, subjects: []string{}},
			}

			for _, subjectOf := range subjectsOf {
				subjects := []string{}
				for subject, err := range client.ReadSubjects(context.Background(), subjectOf.subject, eventsourcingdb.ReadSubjectsOptions{Recursive: subjectOf.recursive}) {
					require.NoError(t, err)
					subjects = append(subjects, subject)
				}

				assert.Equal(t, subjectOf.subjects, subjects, "%s, recursive: %t", subjectOf.subject, subjectOf.recursive)

				// The v1 API always reads recursively, from the base subject on.
				assert.JSONEq(t, `{"baseSubject":"`+subjectOf.subject+`"}`, bodies[len(bodies)-1])
			}
		})

		t.Run("tells an event type that has events from one that is a phantom", func(t *testing.T) {
			for isPhantom, hasEvents := range map[string]bool{"true": false, "false": true} {
				client := clientOfAServerThatAnswers(t, apiV1, http.StatusOK, `{"eventType":"io.eventsourcingdb.test","isPhantom":`+isPhantom+`}`)

				eventType, err := client.ReadEventType(context.Background(), "io.eventsourcingdb.test")

				require.NoError(t, err)
				assert.Equal(t, eventsourcingdb.EventType{Type: "io.eventsourcingdb.test", HasEvents: hasEvents}, eventType)
			}
		})
	})
}

// candidate returns an event candidate, as requestsOf writes it.
func candidate() eventsourcingdb.EventCandidate {
	return eventsourcingdb.EventCandidate{
		Source:  "https://www.eventsourcingdb.io",
		Subject: "/test",
		Type:    "io.eventsourcingdb.test",
		Data:    map[string]int{"value": 23},
	}
}

// eventOfV2 returns an event with the given ID as the v2 API sends it, with
// null for every field it does not have.
func eventOfV2(id string) string {
	return `{"specversion":"1.0","id":"` + id + `","source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","time":"2026-10-01T12:00:00Z","predecessorhash":"0000000000000000000000000000000000000000000000000000000000000000","datacontenttype":"application/json","data":{"value":23},"traceparent":null,"tracestate":null,"hash":"esdb:hash:v2:0000000000000000000000000000000000000000000000000000000000000000","signature":null}`
}

// answerAsServer answers a request as a server with the API that its path
// belongs to, with the shortest answer that succeeds.
func answerAsServer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Server", "EventSourcingDB/test")

	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		answerAsServerOfV1(w, r)
		return
	}

	answerAsServerOfV2(w, r)
}

func answerAsServerOfV1(w http.ResponseWriter, r *http.Request) {
	event := `{"specversion":"1.0","id":"0","time":"2026-10-01T12:00:00Z","source":"https://www.eventsourcingdb.io","subject":"/test","type":"io.eventsourcingdb.test","datacontenttype":"application/json","data":{"value":23},"hash":"","predecessorhash":""}`

	answers := map[string]string{
		"/api/v1/ping":                  apiV1.pingAnswer(),
		"/api/v1/verify-api-token":      `{"type":"io.eventsourcingdb.api.api-token-verified"}`,
		"/api/v1/write-events":          `[` + event + `]`,
		"/api/v1/register-event-schema": `{"type":"io.eventsourcingdb.api.event-schema-registered"}`,
		"/api/v1/read-event-type":       `{"eventType":"io.eventsourcingdb.test","isPhantom":true,"schema":{"type":"object"}}`,
		"/api/v1/read-events":           apiV1.line("event", event),
		"/api/v1/observe-events":        apiV1.line("event", event),
		"/api/v1/run-eventql-query":     apiV1.line("row", event),
		"/api/v1/read-subjects":         apiV1.line("subject", `{"subject":"/"}`),
		"/api/v1/read-event-types":      apiV1.line("eventType", `{"eventType":"io.eventsourcingdb.test","isPhantom":false}`),
	}

	answer, ok := answers[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}

	_, _ = w.Write([]byte(answer + "\n"))
}

func answerAsServerOfV2(w http.ResponseWriter, r *http.Request) {
	event := eventOfV2("0")

	answers := map[string][]string{
		"GET /api/v2/system/ping":                 {apiV2.pingAnswer()},
		"POST /api/v2/system/verify-token":        {apiV2.line("event", `{"type":"io.eventsourcingdb.api.token-verified"}`)},
		"POST /api/v2/data/write-events":          {apiV2.heartbeat(), apiV2.line("event", event), apiV2.end()},
		"POST /api/v2/data/register-event-schema": {apiV2.heartbeat(), apiV2.line("event", `{"type":"io.eventsourcingdb.api.event-schema-registered"}`), apiV2.end()},
		"QUERY /api/v2/data/read-event-type":      {apiV2.line("eventType", `{"type":"io.eventsourcingdb.test","hasEvents":false,"schema":{"type":"object"}}`)},
		"QUERY /api/v2/data/read-events":          {apiV2.heartbeat(), apiV2.line("event", event), apiV2.end()},
		"QUERY /api/v2/data/observe-events":       {apiV2.heartbeat(), apiV2.line("event", event)},
		"QUERY /api/v2/data/run-eventql-query":    {apiV2.line("row", `{"value":`+event+`}`), apiV2.end()},
		"QUERY /api/v2/data/read-subjects":        {apiV2.line("subject", `{"subject":"/"}`), apiV2.end()},
		"QUERY /api/v2/data/read-event-types":     {apiV2.line("eventType", `{"type":"io.eventsourcingdb.test","hasEvents":true,"schema":null}`), apiV2.end()},
	}

	lines, ok := answers[r.Method+" "+r.URL.Path]
	if !ok {
		// A route of the v2 API that a request names with another method
		// answers with 405, and every other path with 404.
		for route := range answers {
			if strings.HasSuffix(route, " "+r.URL.Path) {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
		}

		http.NotFound(w, r)
		return
	}

	for _, line := range lines {
		_, _ = w.Write([]byte(line + "\n"))
	}
}
