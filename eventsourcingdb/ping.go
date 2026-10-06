package eventsourcingdb

import (
	"context"
	"errors"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func (c *Client) Ping(ctx context.Context) error {
	pingURL, err := c.getURL("/api/v1/ping")
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL.String(), nil)
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	err = validateServerHeader(response)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return newDBAPIError("ping", response)
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
