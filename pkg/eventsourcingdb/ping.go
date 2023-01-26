package eventsourcingdb

import (
	"fmt"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"io"
	"net/http"
)

func (client *Client) Ping() error {
	httpClient := &http.Client{
		Timeout: client.configuration.timeout,
	}
	url := client.configuration.baseURL.JoinPath("ping")

	response, err := httpClient.Get(url.String())
	if err != nil {
		return errors.NewServerError("server did not respond")
	}
	if response.StatusCode != http.StatusOK {
		return errors.NewServerError(fmt.Sprintf("server responded with an unexpected status: %s", response.Status))
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return errors.NewServerError("failed to read response body")
	}
	if string(data) != "OK" {
		return errors.NewServerError(fmt.Sprintf("server responded with an unexpected response body: %s", data))
	}

	return nil
}
