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
// well. Once the server answers a request through the v2 API with 404 or 405,
// the client forgets the answer, and the next request asks again.
func (c *Client) chooseAPI(ctx context.Context, action string) (api, error) {
	version, generation, err := c.apiChoice.choose(ctx, c.probeAPI, false)

	return c.apiOf(version, generation, err, action)
}

// chooseAPIAskingAgainIfV1 returns the API to send a request for the given
// action through, as chooseAPI does, but asks the server again if the client
// found out that it offers the v1 API only, rather than taking that answer
// (see apiChoice.choose). A write with a recursive precondition chooses so,
// since only the v2 API can check such a precondition, and the server may
// have been upgraded since the client asked, e.g. while the application runs.
func (c *Client) chooseAPIAskingAgainIfV1(ctx context.Context, action string) (api, error) {
	version, generation, err := c.apiChoice.choose(ctx, c.probeAPI, true)

	return c.apiOf(version, generation, err, action)
}

// apiOf returns the API of the given version, which was chosen with the given
// generation, or the given error of choosing it, as an error of a request for
// the given action.
func (c *Client) apiOf(version apiVersion, generation uint64, err error, action string) (api, error) {
	if err != nil {
		return nil, errorOfProbeFor(err, action)
	}

	if version == apiVersionV2 {
		return v2API{client: c, generation: generation}, nil
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

	// askingAgain is the probe that asks the server again while it is known
	// to offer the v1 API only, for the requests that do not take that answer
	// (see choose). It is apart from running, since every other request goes
	// on with the known answer meanwhile, rather than waiting for it.
	askingAgain *probe

	// generation counts the answers that probes remembered, so that forget
	// forgets only the answer that a request was sent by, and never a newer
	// one.
	generation uint64

	// waiting is called whenever a request starts to wait for the probe of
	// another request. It is nil, but in tests.
	waiting func()
}

// forget forgets that the server offers the v2 API, as the probe of the given
// generation found out, so that the next request asks the server again. It
// leaves the answer of a newer probe alone, and a probe that runs right now,
// as the request that forgets was sent before either, and so tells nothing
// about them.
func (a *apiChoice) forget(generation uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.version == apiVersionV2 && a.generation == generation {
		a.version = apiVersionUnknown
	}
}

// run runs the given probe with find, remembers its outcome if it tells which
// API the server offers, and otherwise keeps what it knew, empties the given
// slot that holds the probe, and lets the requests that wait for it go on,
// also if find panics, in which case they ask themselves.
func (a *apiChoice) run(ctx context.Context, slot **probe, running *probe, find func(ctx context.Context) (apiVersion, error)) {
	hasReturned := false
	defer func() {
		if !hasReturned {
			running.abandoned = true
		}

		a.mu.Lock()
		if hasReturned && running.err == nil {
			a.generation++
			a.version = running.version
			running.generation = a.generation
		}
		*slot = nil
		a.mu.Unlock()
		close(running.done)
	}()

	running.version, running.err = find(ctx)
	running.abandoned = running.err != nil && ctx.Err() != nil
	hasReturned = true
}

// probe is a request that asks the server for its API. Once done is closed,
// version and err hold its outcome, generation the generation with which it
// was remembered, and abandoned tells whether it failed because the context
// of the request that sent it ended, which says nothing about the server.
type probe struct {
	done       chan struct{}
	version    apiVersion
	generation uint64
	err        error
	abandoned  bool
}

// choose returns the API that the server offers, and the generation of that
// answer, as the given function finds out with the given context, unless it
// is known already, or unless another request is finding it out right now, in
// which case it waits for that.
//
// If asksAgainIfV1 is set, a known answer that the server offers the v1 API
// only does not count: the request asks again, in a probe of its own, which
// only the requests that ask again share. Every other request goes on with
// the known answer meanwhile, and the answer of the probe replaces it only if
// it tells for sure which API the server offers. A request whose context has
// already ended asks nothing, and so keeps the known answer as well.
func (a *apiChoice) choose(ctx context.Context, find func(ctx context.Context) (apiVersion, error), asksAgainIfV1 bool) (apiVersion, uint64, error) {
	for {
		err := ctx.Err()
		if err != nil {
			return apiVersionUnknown, 0, err
		}

		a.mu.Lock()
		slot := &a.running
		switch {
		case a.version == apiVersionV1 && asksAgainIfV1:
			slot = &a.askingAgain
		case a.version != apiVersionUnknown:
			version, generation := a.version, a.generation
			a.mu.Unlock()

			return version, generation, nil
		}

		running := *slot
		if running == nil {
			running = &probe{done: make(chan struct{})}
			*slot = running
			a.mu.Unlock()

			a.run(ctx, slot, running, find)

			return running.version, running.generation, running.err
		}

		waiting := a.waiting
		a.mu.Unlock()

		if waiting != nil {
			waiting()
		}

		select {
		case <-running.done:
		case <-ctx.Done():
			return apiVersionUnknown, 0, ctx.Err()
		}

		// The request that sent the probe ended, so this one asks itself.
		if running.abandoned {
			continue
		}

		return running.version, running.generation, running.err
	}
}
