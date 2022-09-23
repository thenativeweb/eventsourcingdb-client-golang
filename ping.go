package eventsourcingdb

import (
	"errors"
	"io"
	"net/http"
)

var ErrPingFailed = errors.New("ping failed")

func (client *Client) Ping() error {
	httpClient := &http.Client{
		Timeout: client.configuration.timeout,
	}
	url := client.configuration.baseURL + "/ping"

	response, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return ErrPingFailed
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return ErrPingFailed
	}
	if string(data) != "OK" {
		return ErrPingFailed
	}

	return nil
}
