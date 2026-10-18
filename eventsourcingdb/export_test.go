package eventsourcingdb

import (
	"net/http"
	"time"
)

// HeartbeatTimeoutAfter returns the error with which a stream with heartbeats
// ends if nothing arrives for the given stall timeout, so that tests need not
// wait for 30 seconds to see it.
var HeartbeatTimeoutAfter = heartbeatTimeoutAfter

// StallTimeoutOf returns the stall timeout of a client.
func StallTimeoutOf(client *Client) time.Duration {
	return client.stallTimeout
}

// HTTPClientOf returns the HTTP client that a client sends its requests with.
func HTTPClientOf(client *Client) *http.Client {
	return client.httpClient
}
