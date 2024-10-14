package eventsourcingdb

import (
	"net/url"
	"strconv"

	"github.com/thenativeweb/eventsourcingdb-client-golang/errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/configuration"
)

type Client struct {
	configuration configuration.ClientConfiguration
}

func NewClient(baseURL string, accessToken string) (Client, error) {
	if strconv.IntSize != 64 {
		return Client{}, errors.NewClientError("64-bit architecture required")
	}
	if accessToken == "" {
		return Client{}, errors.NewInvalidParameterError("AccessToken", "the access token must not be empty")
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return Client{}, errors.NewInvalidParameterError("baseURL", err.Error())
	}
	if parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https" {
		return Client{}, errors.NewInvalidParameterError("baseURL", "must use HTTP or HTTPS")
	}

	clientConfiguration := configuration.GetDefaultConfiguration(parsedBaseURL, accessToken)

	client := Client{clientConfiguration}

	return client, nil
}
