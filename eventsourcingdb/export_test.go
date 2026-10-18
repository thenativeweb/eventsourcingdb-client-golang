package eventsourcingdb

import (
	"net/http"
	"time"
)

// ConnectionStalledAfter returns the error with which a request ends if
// nothing arrives for the given stall timeout, so that tests need not wait for
// 30 seconds to see it.
func ConnectionStalledAfter(stallTimeout time.Duration) error {
	return &connectionStalledError{stallTimeout: stallTimeout}
}

// StallTimeoutOf returns the stall timeout of a client.
func StallTimeoutOf(client *Client) time.Duration {
	return client.stallTimeout
}

// HTTPClientOf returns the HTTP client that a client sends its requests with.
func HTTPClientOf(client *Client) *http.Client {
	return client.httpClient
}
