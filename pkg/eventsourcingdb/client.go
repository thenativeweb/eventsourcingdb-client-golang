package eventsourcingdb

import (
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/configuration"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"net/url"
	"strconv"
)

type Client struct {
	configuration configuration.ClientConfiguration
}

func NewClient(baseURL string, accessToken string, options ...ClientOption) (Client, error) {
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

	for _, option := range options {
		if err := option.apply(&clientConfiguration); err != nil {
			return Client{}, errors.NewInvalidParameterError(option.name, err.Error())
		}
	}

	client := Client{clientConfiguration}

	return client, nil
}
