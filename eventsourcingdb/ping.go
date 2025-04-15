package eventsourcingdb

import (
	"errors"
	"fmt"
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

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to ping, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK)
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
