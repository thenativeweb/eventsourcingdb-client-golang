package eventsourcingdb

import (
	"context"
	"errors"
	"io"
	"iter"
	"time"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

// ErrHeartbeatTimeout reports that a stream ended because neither an event
// nor a heartbeat arrived for 30 seconds. While there is nothing else to send,
// the server sends a heartbeat every second on the streams of ObserveEvents
// and RunEventQLQuery, so a stream that stays silent for that long has
// stalled, e.g. behind a proxy that keeps the connection open but no longer
// passes anything on. The client then closes the connection. Use errors.Is to
// check for it, which also finds it if it is wrapped:
//
//	for event, err := range client.ObserveEvents(ctx, "/books/42", options) {
//		if errors.Is(err, eventsourcingdb.ErrHeartbeatTimeout) {
//			// ...
//		}
//	}
var ErrHeartbeatTimeout = errors.New("no event and no heartbeat arrived for 30 seconds")

// heartbeatTimeout is how long a stream with heartbeats waits for its next
// line. It is a variable only so that tests can shorten it.
var heartbeatTimeout = 30 * time.Second

// unmarshalNDJSONWithHeartbeatTimeout reads the lines of a stream on which the
// server sends heartbeats, as internal.UnmarshalNDJSON does. If no line
// arrives within heartbeatTimeout, it cancels ctx with ErrHeartbeatTimeout as
// the cause, which closes the connection of the request that ctx belongs to,
// and ends with ErrHeartbeatTimeout. Only the time spent waiting for a line
// counts, not the time the caller spends on one.
func unmarshalNDJSONWithHeartbeatTimeout(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	r io.Reader,
) iter.Seq2[internal.Line, error] {
	return func(yield func(internal.Line, error) bool) {
		timer := time.AfterFunc(heartbeatTimeout, func() {
			cancel(ErrHeartbeatTimeout)
		})
		defer timer.Stop()

		for line, err := range internal.UnmarshalNDJSON(ctx, r) {
			timer.Stop()

			if err != nil {
				// Once the timer has canceled the context, reading fails with
				// whatever error the transport or the check of the context
				// reports for that, so we report the cause instead. A context
				// that the caller ended keeps its own cause, so its error is
				// reported as before.
				if errors.Is(context.Cause(ctx), ErrHeartbeatTimeout) {
					err = ErrHeartbeatTimeout
				}

				yield(internal.Line{}, err)
				return
			}

			if !yield(line, nil) {
				return
			}

			timer.Reset(heartbeatTimeout)
		}
	}
}
