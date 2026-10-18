package eventsourcingdb

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

// methodQuery is the HTTP method QUERY (RFC 10008), with which the v2 API
// reads. net/http has no constant for it.
const methodQuery = "QUERY"

// request sends a request with the given method to the given path of the
// server, with the context of the guard, with the given body as JSON unless
// it is nil, and with the API token if authorize is true. It returns the
// response once its header has arrived, unless the response does not come
// from EventSourcingDB. The caller has to close its body.
func (c *Client) request(guard *stallGuard, method string, path string, body any, authorize bool) (*http.Response, error) {
	urlPath, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	targetURL := c.baseURL.ResolveReference(urlPath)

	var bodyReader io.Reader
	if body != nil {
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		bodyReader = bytes.NewReader(bodyJSON)
	}

	request, err := http.NewRequestWithContext(guard.ctx, method, targetURL.String(), bodyReader)
	if err != nil {
		return nil, err
	}
	if authorize {
		request.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.send(guard, request)
	if err != nil {
		return nil, err
	}

	err = validateServerHeader(response)
	if err != nil {
		response.Body.Close()
		return nil, err
	}

	return response, nil
}
