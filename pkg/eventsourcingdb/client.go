package eventsourcingdb

import (
	"github.com/Masterminds/semver"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"net/url"
	"strconv"
)

type clientConfiguration struct {
	baseURL         *url.URL
	accessToken     string
	protocolVersion semver.Version
	maxTries        int
}

func getDefaultConfiguration(baseURL *url.URL, accessToken string) clientConfiguration {
	return clientConfiguration{
		baseURL:         baseURL,
		accessToken:     accessToken,
		protocolVersion: *semver.MustParse("1.0.0"),
		maxTries:        10,
	}
}

type Client struct {
	configuration clientConfiguration
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

	configuration := getDefaultConfiguration(parsedBaseURL, accessToken)

	for _, option := range options {
		if err := option.apply(&configuration); err != nil {
			return Client{}, errors.NewInvalidParameterError(option.name, err.Error())
		}
	}

	client := Client{configuration}

	return client, nil
}
