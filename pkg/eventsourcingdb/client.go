package eventsourcingdb

import (
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"net/url"
	"strconv"
	"time"

	"github.com/Masterminds/semver"
)

type clientConfiguration struct {
	baseURL         url.URL
	timeout         time.Duration
	accessToken     string
	protocolVersion semver.Version
	maxTries        int
}

func getDefaultConfiguration(baseURL url.URL) clientConfiguration {
	return clientConfiguration{
		baseURL:         baseURL,
		timeout:         10 * time.Second,
		accessToken:     "",
		protocolVersion: *semver.MustParse("1.0.0"),
		maxTries:        10,
	}
}

type Client struct {
	configuration clientConfiguration
}

func NewClient(baseURL string, options ...ClientOption) (Client, error) {
	if strconv.IntSize != 64 {
		return Client{}, errors.NewClientError("64-bit architecture required")
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return Client{}, errors.NewInvalidParameterError("baseURL", err.Error())
	}
	if parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https" {
		return Client{}, errors.NewInvalidParameterError("baseURL", "must use HTTP or HTTPS")
	}

	configuration := getDefaultConfiguration(*parsedBaseURL)

	for _, option := range options {
		if err := option.apply(&configuration); err != nil {
			return Client{}, errors.NewInvalidParameterError(option.name, err.Error())
		}
	}

	client := Client{configuration}

	return client, nil
}
