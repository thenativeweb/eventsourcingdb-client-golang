package eventsourcingdb

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Client talks to an EventSourcingDB. Create it with NewClient. It is safe for
// concurrent use.
//
// The client speaks the v2 API of the server if the server offers it, and the
// v1 API otherwise. Creating it sends no request: the first request of the
// client finds out which API the server offers, and the client remembers that
// for every request after it.
type Client struct {
	baseURL      *url.URL
	apiToken     string
	stallTimeout time.Duration
	httpClient   *http.Client
	apiChoice    apiChoice
}

// ClientOption configures a client. Hand options to NewClient, e.g.
// WithStallTimeout.
type ClientOption func(client *Client) error

// WithStallTimeout sets how long a request of the client waits for anything to
// arrive from the server before it fails. The default is 30 seconds.
//
// The clock starts when a request is sent, so it covers connecting and waiting
// for the response, and starts again with every byte that arrives, e.g. with
// every event or heartbeat of a stream. It does not limit how long a request
// takes in total: a stream that keeps receiving events or heartbeats never
// ends because of it. The time the caller spends on an item of a stream does
// not count either.
//
// A request that stalls fails with ErrConnectionStalled. While there is
// nothing else to send, the server sends a heartbeat every second on its
// streams, so the stall timeout should be several seconds at least. To limit a
// single request further, hand it a context with a deadline.
//
// The stall timeout must be greater than zero, or NewClient returns an error.
func WithStallTimeout(stallTimeout time.Duration) ClientOption {
	return func(client *Client) error {
		if stallTimeout <= 0 {
			return errors.New("stall timeout must be greater than zero")
		}

		client.stallTimeout = stallTimeout
		return nil
	}
}

// NewClient returns a client of the EventSourcingDB at the given base URL,
// which uses the given API token, and is configured by the given options.
func NewClient(baseURL *url.URL, apiToken string, options ...ClientOption) (*Client, error) {
	client := &Client{
		baseURL:      baseURL,
		apiToken:     apiToken,
		stallTimeout: defaultStallTimeout,
	}

	for _, option := range options {
		if option == nil {
			continue
		}

		err := option(client)
		if err != nil {
			return nil, err
		}
	}

	client.httpClient = newHTTPClient(client.stallTimeout)

	return client, nil
}

// newHTTPClient returns the HTTP client that a client sends its requests
// with. Every client has one of its own rather than using http.DefaultClient,
// so that an application that changes http.DefaultClient, e.g. by setting a
// Timeout that would cut every stream short, does not change how the client
// talks to the database. It has no Timeout of its own for the same reason: the
// stall timeout bounds every request instead (see stallGuard).
func newHTTPClient(stallTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: newTransport(stallTimeout),
	}
}

// newTransport returns a copy of http.DefaultTransport, so that the client
// uses what an application configured there, e.g. a proxy or the certificates
// to trust, without sharing the transport itself. If an application replaced
// http.DefaultTransport by a transport of another type, which can not be
// copied, it returns a transport that is configured as http.DefaultTransport
// is by default.
//
// Connecting and the TLS handshake get the stall timeout as their timeout
// rather than the 30 and 10 seconds of http.DefaultTransport, and waiting for
// the header of a response gets none, so that it is the stall timeout that
// decides when a request gives up, also if it is longer than those. Connecting
// and the TLS handshake go on in the background once a request has given up,
// as net/http keeps a connection for the next request, so their timeout keeps
// them from going on for longer than that.
func newTransport(stallTimeout time.Duration) *http.Transport {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}

	transport.DialContext = (&net.Dialer{
		Timeout:   stallTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = stallTimeout
	transport.ResponseHeaderTimeout = 0

	return transport
}
