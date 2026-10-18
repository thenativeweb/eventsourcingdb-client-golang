package eventsourcingdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"sync"
)

// api is the API of the server that a client talks to. Every function of the
// client that sends a request chooses it first, with chooseAPI, and then sends
// the request through it, so that the same code runs against a server with
// the v2 API (v2API) and against one with the v1 API only (v1API).
type api interface {
	ping(ctx context.Context) error
	verifyAPIToken(ctx context.Context) error
	writeEvents(ctx context.Context, body writeEventsRequestBody) ([]Event, error)
	registerEventSchema(ctx context.Context, eventType string, schema map[string]any) error
	readEventType(ctx context.Context, eventType string) (EventType, error)
	readEvents(ctx context.Context, body readEventsRequestBody) iter.Seq2[Event, error]
	observeEvents(ctx context.Context, body observeEventsRequestBody) iter.Seq2[Event, error]
	runEventQLQuery(ctx context.Context, query string) iter.Seq2[json.RawMessage, error]
	readSubjects(ctx context.Context, subject string, options ReadSubjectsOptions) iter.Seq2[string, error]
	readEventTypes(ctx context.Context) iter.Seq2[EventType, error]
}

// apiVersion names an API of the server.
type apiVersion int

const (
	// apiVersionUnknown is the API of a server that the client has not found
	// out yet.
	apiVersionUnknown apiVersion = iota
	apiVersionV1
	apiVersionV2
)

// pathOfV2Ping is the path that the client asks to find out whether the
// server offers the v2 API. A server without it answers with 404 or 405.
const pathOfV2Ping = "/api/v2/system/ping"

// chooseAPI returns the API to send a request for the given action through.
// On the first request, it asks the server whether it offers the v2 API, and
// remembers the answer, if it is a definitive one. Otherwise, the request
// fails with the error of asking, and the next one asks again. Requests that
// start while the server is asked wait for its answer, rather than asking as
// well.
func (c *Client) chooseAPI(ctx context.Context, action string) (api, error) {
	version, err := c.apiChoice.choose(ctx, c.probeAPI)
	if err != nil {
		return nil, errorOfProbeFor(err, action)
	}

	if version == apiVersionV2 {
		return v2API{client: c}, nil
	}

	return v1API{client: c}, nil
}

// probeAPI asks the server whether it offers the v2 API, with a request to
// pathOfV2Ping. An event that answers the ping means that it does, and status
// 404 or 405 that it does not, since a server without the v2 API answers
// unknown paths of the v2 API so. Every other outcome is no answer, and is
// returned as an error.
//
// A status of 404 or 405 means the v1 API also if it does not come from
// EventSourcingDB, so that a server with the v1 API only keeps working behind
// a proxy that passes on nothing but its paths, and answers every other path
// itself. Every request that follows checks that the server is EventSourcingDB
// anyway.
func (c *Client) probeAPI(ctx context.Context) (apiVersion, error) {
	guard := c.guard(ctx)
	defer guard.end()

	response, err := c.requestOfAnyServer(guard, http.MethodGet, pathOfV2Ping, nil, false)
	if err != nil {
		return apiVersionUnknown, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
		return apiVersionV1, nil
	}

	err = validateServerHeader(response)
	if err != nil {
		return apiVersionUnknown, err
	}

	if response.StatusCode != http.StatusOK {
		return apiVersionUnknown, newDBAPIError("", response)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return apiVersionUnknown, err
	}

	var answer struct {
		Type string `json:"type"`
		Data struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &answer)
	if err != nil || answer.Type != "event" || answer.Data.Type != typeOfPingReceived {
		return apiVersionUnknown, &unexpectedProbeAnswerError{}
	}

	return apiVersionV2, nil
}

// unexpectedProbeAnswerError reports that the server answered the request that
// asks for its API with status 200, but not with an event that answers the
// ping, so that it is unclear which API it offers.
type unexpectedProbeAnswerError struct {
	action string
}

func (e *unexpectedProbeAnswerError) Error() string {
	return fmt.Sprintf(
		"failed to %s, as the server answered %s with status 200, but without the event of a ping, so it is unclear whether it offers the v2 API",
		e.action, pathOfV2Ping,
	)
}

// errorOfProbeFor returns the error with which asking the server for its API
// failed, as an error of a request for the given action, since a request that
// waits for the answer of another one names its own action.
func errorOfProbeFor(err error, action string) error {
	switch probeErr := err.(type) {
	case *DBAPIError:
		dbAPIError := *probeErr
		dbAPIError.Action = action

		return &dbAPIError
	case *unexpectedProbeAnswerError:
		return &unexpectedProbeAnswerError{action: action}
	default:
		return err
	}
}

// apiChoice remembers the API that the server offers, once a probe found it
// out, and lets requests that start while a probe runs wait for it.
type apiChoice struct {
	mu      sync.Mutex
	version apiVersion
	running *probe

	// waiting is called whenever a request starts to wait for the probe of
	// another request. It is nil, but in tests.
	waiting func()
}

// probe is a request that asks the server for its API. Once done is closed,
// version and err hold its outcome, and abandoned tells whether it failed
// because the context of the request that sent it ended, which says nothing
// about the server.
type probe struct {
	done      chan struct{}
	version   apiVersion
	err       error
	abandoned bool
}

// choose returns the API that the server offers, as the given function finds
// out with the given context, unless it is known already, or unless another
// request is finding it out right now, in which case it waits for that.
func (a *apiChoice) choose(ctx context.Context, find func(ctx context.Context) (apiVersion, error)) (apiVersion, error) {
	for {
		err := ctx.Err()
		if err != nil {
			return apiVersionUnknown, err
		}

		a.mu.Lock()
		if a.version != apiVersionUnknown {
			version := a.version
			a.mu.Unlock()

			return version, nil
		}

		running := a.running
		if running == nil {
			running = &probe{done: make(chan struct{})}
			a.running = running
			a.mu.Unlock()

			running.version, running.err = find(ctx)
			running.abandoned = running.err != nil && ctx.Err() != nil

			a.mu.Lock()
			if running.err == nil {
				a.version = running.version
			}
			a.running = nil
			a.mu.Unlock()
			close(running.done)

			return running.version, running.err
		}

		waiting := a.waiting
		a.mu.Unlock()

		if waiting != nil {
			waiting()
		}

		select {
		case <-running.done:
		case <-ctx.Done():
			return apiVersionUnknown, ctx.Err()
		}

		// The request that sent the probe ended, so this one asks itself.
		if running.abandoned {
			continue
		}

		return running.version, running.err
	}
}
