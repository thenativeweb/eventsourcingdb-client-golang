package eventsourcingdb

import (
	"errors"
	"io"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/network"
)

var ErrPingFailed = errors.New("ping failed")

func (client *Client) Ping() error {
	httpClient := &http.Client{
		Timeout: client.configuration.timeout,
	}
	url := client.configuration.baseUrl + "/ping"

	response, err := network.Retry(func() (*http.Response, error) {
		return httpClient.Get(url)
	}, 10)
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
