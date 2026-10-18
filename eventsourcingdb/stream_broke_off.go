package eventsourcingdb

import "fmt"

// errStreamBrokeOff reports that a stream ended before it was complete: a
// stream of the v2 API that ends on its own before its end item, and a stream
// that never ends, i.e. one of observed events, at all, without an error. The
// server ends a stream so only after a failure it does not report, e.g. one it
// merely logs, so what the stream handed out may be incomplete. A server that
// shuts down reports that with an error item instead.
func errStreamBrokeOff(action string) error {
	return fmt.Errorf("failed to %s, the stream broke off", action)
}
