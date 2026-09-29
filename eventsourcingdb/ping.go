package eventsourcingdb

import (
	"errors"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func (c *Client) Ping() error {
	pingURL, err := c.getURL("/api/v1/ping")
	if err != nil {
		return err
	}

	response, err := http.Get(pingURL.String())
	if err != nil {
		return err
	}
	defer response.Body.Close()

	err = internal.ValidateServerHeader(response)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return internal.NewStatusError("ping", response)
	}

	type Result struct {
		Type string `json:"type"`
	}

	var result Result
	err = internal.ParseJSON(response.Body, &result)
	if err != nil {
		return err
	}

	if result.Type != "io.eventsourcingdb.api.ping-received" {
		return errors.New("failed to ping")
	}

	return nil
}
