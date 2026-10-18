package eventsourcingdb

import (
	"errors"
)

// ErrHeartbeatTimeout reports that a stream ended because neither an event
// nor a heartbeat arrived for the stall timeout of the client, 30 seconds by
// default (see WithStallTimeout). While there is nothing else to send, the
// server sends a heartbeat every second on the streams of ObserveEvents and
// RunEventQLQuery, so a stream that stays silent for that long has stalled,
// e.g. behind a proxy that keeps the connection open but no longer passes
// anything on. The client then closes the connection. With the default stall
// timeout, the stream ends with ErrHeartbeatTimeout itself, and with any other
// one, with an error that wraps it and names the stall timeout. Use errors.Is
// to check for it, which also finds it if it is wrapped:
//
//	for event, err := range client.ObserveEvents(ctx, "/books/42", options) {
//		if errors.Is(err, eventsourcingdb.ErrHeartbeatTimeout) {
//			// ...
//		}
//	}
//
// Before the server has begun to answer, a stall ends these streams with
// ErrConnectionStalled, as every other request.
var ErrHeartbeatTimeout = errors.New("no event and no heartbeat arrived for 30 seconds")
