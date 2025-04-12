package eventsourcingdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	baseURL  *url.URL
	apiToken string
}

func NewClient(baseURL *url.URL, apiToken string) (*Client, error) {
	client := &Client{
		baseURL,
		apiToken,
	}

	return client, nil
}

func (c *Client) getURL(path string) (*url.URL, error) {
	urlPath, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	targetURL := c.baseURL.ResolveReference(urlPath)
	if err != nil {
		return nil, err
	}

	return targetURL, nil
}

func (c *Client) Ping() error {
	pingURL, err := c.getURL("/api/v1/ping")

	response, err := http.Get(pingURL.String())
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to ping, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK)
	}

	type ResponseBody struct {
		Type string `json:"type"`
	}

	var responseBody ResponseBody
	err = json.NewDecoder(response.Body).Decode(&responseBody)
	if err != nil {
		return err
	}

	if responseBody.Type != "io.eventsourcingdb.api.ping-received" {
		return errors.New("failed to ping")
	}

	return nil
}

func (c *Client) VerifyAPIToken() error {
	verifyAPITokenURL, err := c.getURL("/api/v1/verify-api-token")

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

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to verify API token, got HTTP status code '%d', expected '%d'", response.StatusCode, http.StatusOK)
	}

	type ResponseBody struct {
		Type string `json:"type"`
	}

	var responseBody ResponseBody
	err = json.NewDecoder(response.Body).Decode(&responseBody)
	if err != nil {
		return err
	}

	if responseBody.Type != "io.eventsourcingdb.api.api-token-verified" {
		return errors.New("failed to verify API token")
	}

	return nil
}
