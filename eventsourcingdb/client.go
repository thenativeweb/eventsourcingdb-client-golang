package eventsourcingdb

import (
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
