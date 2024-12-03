package eventsourcingdb

import (
	"fmt"
	"io"
	"net/http"
)

func (client *Client) Ping() error {
	httpClient := &http.Client{}
	url := client.configuration.BaseURL.JoinPath("api/ping")

	response, err := httpClient.Get(url.String())
	if err != nil {
		return NewServerError("server did not respond")
	}
	if response.StatusCode != http.StatusOK {
		return NewServerError(fmt.Sprintf("server responded with an unexpected status: %s", response.Status))
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return NewServerError("failed to read response body")
	}
	if string(data) != "OK" {
		return NewServerError(fmt.Sprintf("server responded with an unexpected response body: %s", data))
	}

	return nil
}
