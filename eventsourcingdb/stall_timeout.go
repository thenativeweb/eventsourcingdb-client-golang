package eventsourcingdb

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/internal"
)

// ErrConnectionStalled reports that a request ended because nothing arrived
// from the server for the stall timeout of the client, 30 seconds by default
// (see WithStallTimeout), e.g. because the server can not be reached, or
// because a proxy keeps the connection open but no longer passes anything on.
// The client then closes the connection. Every function of the client that
// sends a request returns an error in that case that errors.Is finds it in,
// and that names the stall timeout, e.g. "connection stalled, nothing arrived
// from the server for 30 seconds". Only observing events and running an
// EventQL query end with ErrHeartbeatTimeout instead once the server has begun
// to answer. Use errors.Is to check for it, which also finds it if it is
// wrapped:
//
//	_, err := client.WriteEvents(ctx, events, preconditions)
//	if errors.Is(err, eventsourcingdb.ErrConnectionStalled) {
//		// ...
//	}
//
// Since a stalled connection is a timeout, errors.Is also finds
// context.DeadlineExceeded in the error, as it does in the error of a timeout
// of an http.Client, and the error has a Timeout method that returns true.
var ErrConnectionStalled = errors.New("connection stalled")

// defaultStallTimeout is the stall timeout of a client unless WithStallTimeout
// sets another one.
const defaultStallTimeout = 30 * time.Second

// errStalled is the cause with which a stall guard ends the context of its
// request, so that the guard can tell a stall from any other end of the
// context.
var errStalled = errors.New("stalled")

// connectionStalledError reports a stall of a request with ErrConnectionStalled,
// and names how long nothing arrived.
type connectionStalledError struct {
	stallTimeout time.Duration
}

func (e *connectionStalledError) Error() string {
	return fmt.Sprintf("%s, nothing arrived from the server for %s", ErrConnectionStalled, describeDuration(e.stallTimeout))
}

func (e *connectionStalledError) Is(target error) bool {
	return target == ErrConnectionStalled || target == context.DeadlineExceeded
}

// Timeout reports that the error is a timeout, as the errors of a timeout of
// net/http do.
func (e *connectionStalledError) Timeout() bool {
	return true
}

// heartbeatTimeoutError reports a stall of a stream with heartbeats with
// ErrHeartbeatTimeout, and names how long nothing arrived, if that is not the
// default stall timeout that ErrHeartbeatTimeout names.
type heartbeatTimeoutError struct {
	stallTimeout time.Duration
}

func (e *heartbeatTimeoutError) Error() string {
	return "no event and no heartbeat arrived for " + describeDuration(e.stallTimeout)
}

func (e *heartbeatTimeoutError) Is(target error) bool {
	return target == ErrHeartbeatTimeout
}

// heartbeatTimeoutAfter returns the error with which a stream with heartbeats
// ends if nothing arrives for the given stall timeout: ErrHeartbeatTimeout
// itself for the default stall timeout, as before, and an error that wraps it
// and names the stall timeout for every other one.
func heartbeatTimeoutAfter(stallTimeout time.Duration) error {
	if stallTimeout == defaultStallTimeout {
		return ErrHeartbeatTimeout
	}

	return &heartbeatTimeoutError{stallTimeout: stallTimeout}
}

// describeDuration writes a duration in whole seconds as such, e.g. "30
// seconds", and every other duration as time.Duration does, e.g. "250ms".
func describeDuration(duration time.Duration) string {
	if duration%time.Second != 0 {
		return duration.String()
	}

	seconds := int64(duration / time.Second)
	if seconds == 1 {
		return "1 second"
	}

	return fmt.Sprintf("%d seconds", seconds)
}

// stallGuard ends a request once nothing arrives from the server for the
// stall timeout of the client. The clock starts when the guard is created,
// before the request is sent, so it covers connecting and waiting for the
// response as well, and every byte that arrives starts it again: the
// connection being established, the TLS handshake, the first byte and the
// header of the response, and every byte of its body. While the caller handles
// an item of a stream, the clock stops, since only waiting for the server
// counts. A stream that keeps receiving items or heartbeats is never ended.
//
// The guard ends the request by ending its context with errStalled as the
// cause, which closes the connection. Whatever fails then fails because of
// that, so failure reports the stall instead.
type stallGuard struct {
	ctx          context.Context
	cancel       context.CancelCauseFunc
	stallTimeout time.Duration

	// heartbeats tells whether the request reads a stream on which the server
	// sends heartbeats, so that a stall ends it with ErrHeartbeatTimeout. It
	// is only used by the goroutine that sends the request.
	heartbeats bool

	mu      sync.Mutex
	timer   *time.Timer
	waiting bool
	ended   bool
}

// guard starts to guard a request with the given context against stalls. The
// request has to be sent with the context of the guard, and the guard has to
// be ended once the request is over.
func (c *Client) guard(ctx context.Context) *stallGuard {
	ctx, cancel := context.WithCancelCause(ctx)

	guard := &stallGuard{
		cancel:       cancel,
		stallTimeout: c.stallTimeout,
		waiting:      true,
	}
	guard.timer = time.AfterFunc(c.stallTimeout, func() {
		cancel(errStalled)
	})
	guard.ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		ConnectDone: func(_, _ string, err error) {
			if err == nil {
				guard.received()
			}
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err == nil {
				guard.received()
			}
		},
		GotFirstResponseByte: guard.received,
	})

	return guard
}

// received starts the clock again, as something arrived from the server.
func (g *stallGuard) received() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.waiting && !g.ended {
		g.timer.Reset(g.stallTimeout)
	}
}

// pause stops the clock while the caller handles an item of a stream.
func (g *stallGuard) pause() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.waiting = false
	g.timer.Stop()
}

// resume starts the clock again once the caller has handled an item.
func (g *stallGuard) resume() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.ended {
		return
	}

	g.waiting = true
	g.timer.Reset(g.stallTimeout)
}

// end stops guarding the request, and ends its context.
func (g *stallGuard) end() {
	g.mu.Lock()
	g.ended = true
	g.timer.Stop()
	g.mu.Unlock()

	g.cancel(nil)
}

// failure returns err, or the error that reports the stall if a stall ended
// the request. If the context that the caller handed over ended first, it is
// the error of that context that counts, as before.
func (g *stallGuard) failure(err error) error {
	if err == nil || !errors.Is(context.Cause(g.ctx), errStalled) {
		return err
	}

	if g.heartbeats {
		return heartbeatTimeoutAfter(g.stallTimeout)
	}

	return &connectionStalledError{stallTimeout: g.stallTimeout}
}

// send sends a request that has the context of the guard with the HTTP client
// of the client. The body of the response it returns starts the clock again
// with every byte that arrives, and reports a stall while it is read.
func (c *Client) send(guard *stallGuard, request *http.Request) (*http.Response, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, guard.failure(err)
	}

	guard.received()
	response.Body = &guardedBody{body: response.Body, guard: guard}

	return response, nil
}

// guardedBody is the body of a response that a stall guard watches.
type guardedBody struct {
	body  io.ReadCloser
	guard *stallGuard
}

func (b *guardedBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	if n > 0 {
		b.guard.received()
	}
	if err != nil && !errors.Is(err, io.EOF) {
		err = b.guard.failure(err)
	}

	return n, err
}

func (b *guardedBody) Close() error {
	return b.body.Close()
}

// lines reads the lines of a stream, as internal.UnmarshalNDJSON does. The
// clock stops while the caller handles a line, so that only the time spent
// waiting for a line counts. If heartbeats is true, the server sends
// heartbeats on the stream, so a stall ends it with ErrHeartbeatTimeout.
func (g *stallGuard) lines(body io.Reader, heartbeats bool) iter.Seq2[internal.Line, error] {
	return func(yield func(internal.Line, error) bool) {
		g.heartbeats = heartbeats

		for line, err := range internal.UnmarshalNDJSON(g.ctx, body) {
			if err != nil {
				yield(internal.Line{}, g.failure(err))
				return
			}

			g.pause()
			handled := yield(line, nil)
			g.resume()

			if !handled {
				return
			}
		}
	}
}
