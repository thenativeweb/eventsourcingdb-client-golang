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
	"sync/atomic"
	"time"
)

// ErrConnectionStalled reports that a request ended because nothing arrived
// from the server for the stall timeout of the client, 30 seconds by default
// (see WithStallTimeout), e.g. because the server can not be reached, or
// because a proxy keeps the connection open but no longer passes anything on.
// The client then closes the connection. Every function of the client that
// sends a request returns an error in that case that errors.Is finds it in,
// and that names the stall timeout, e.g. "connection stalled, nothing arrived
// from the server for 30 seconds", whether the server has not begun to answer
// yet, or stops in the middle of an answer or of a stream. On a stream, the
// heartbeats that the server sends while there is nothing else to send count
// as something that arrives. Use errors.Is to check for it, which also finds
// it if it is wrapped:
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
//
// The guard also tells whether a connection to the server was established,
// so that a write that fails can tell whether the server may have stored it.
type stallGuard struct {
	ctx          context.Context
	cancel       context.CancelCauseFunc
	stallTimeout time.Duration
	connected    atomic.Bool

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
		GotConn: func(httptrace.GotConnInfo) {
			guard.connected.Store(true)
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

// isConnected tells whether a connection to the server was established for
// the request, so that it may have been sent.
func (g *stallGuard) isConnected() bool {
	return g.connected.Load()
}

// guardedLines reads the lines of a stream that a stall guard watches, and
// decodes each of them as a T, as unmarshalNDJSON does. The clock
// stops while the caller handles a line, so that only the time spent waiting
// for a line counts.
func guardedLines[T any](g *stallGuard, body io.Reader) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for line, err := range unmarshalNDJSON[T](g.ctx, body) {
			if err != nil {
				yield(line, g.failure(err))
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
