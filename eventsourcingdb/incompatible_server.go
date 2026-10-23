package eventsourcingdb

import "errors"

// ErrIncompatibleServer reports that the server does not offer the v2 API,
// which a recursive precondition needs (see PreconditionOptions), since the
// v1 API can not check it. WriteEvents returns it for a write with a
// recursive precondition against such a server, before it sends the write,
// so that nothing is written. Use errors.Is to check for it, which also finds
// it if it is wrapped:
//
//	_, err := client.WriteEvents(ctx, events, preconditions)
//	if errors.Is(err, eventsourcingdb.ErrIncompatibleServer) {
//		// ...
//	}
var ErrIncompatibleServer = errors.New("the server does not offer the v2 API, which a recursive precondition needs")
