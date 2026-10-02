package eventsourcingdb

import (
	"errors"
	"net/http"

	"github.com/thenativeweb/eventsourcingdb-client-golang/internal"
)

func (c *Client) VerifyAPIToken() error {
	verifyAPITokenURL, err := c.getURL("/api/v1/verify-api-token")
	if err != nil {
		return err
	}

	request := &http.Request{
		Method: http.MethodPost,
		URL:    verifyAPITokenURL,
		Header: http.Header{
			"Authorization": []string{"Bearer " + c.apiToken},
		},
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
		return newDBAPIError("verify API token", response)
	}

	type Result struct {
		Type string `json:"type"`
	}

	var result Result
	err = internal.ParseJSON(response.Body, &result)
	if err != nil {
		return err
	}

	if result.Type != "io.eventsourcingdb.api.api-token-verified" {
		return errors.New("failed to verify API token")
	}

	return nil
}
