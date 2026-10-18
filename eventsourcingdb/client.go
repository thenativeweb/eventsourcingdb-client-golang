package eventsourcingdb

import (
	"net"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    *url.URL
	apiToken   string
	httpClient *http.Client
}

func NewClient(baseURL *url.URL, apiToken string) (*Client, error) {
	client := &Client{
		baseURL:    baseURL,
		apiToken:   apiToken,
		httpClient: newHTTPClient(),
	}

	return client, nil
}

// newHTTPClient returns the HTTP client that a client sends its requests
// with. Every client has one of its own rather than using http.DefaultClient,
// so that an application that changes http.DefaultClient, e.g. by setting a
// Timeout that would cut every stream short, does not change how the client
// talks to the database. It has no Timeout of its own for the same reason.
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: newTransport(),
	}
}

// newTransport returns a copy of http.DefaultTransport, so that the client
// uses what an application configured there, e.g. a proxy or the certificates
// to trust, without sharing the transport itself. If an application replaced
// http.DefaultTransport by a transport of another type, which can not be
// copied, it returns a transport that is configured as http.DefaultTransport
// is by default.
func newTransport() *http.Transport {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		return defaultTransport.Clone()
	}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
