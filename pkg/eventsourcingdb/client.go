package eventsourcingdb

import (
	"strconv"
	"time"

	"github.com/Masterminds/semver"
)

type clientConfiguration struct {
	baseURL         string
	timeout         time.Duration
	accessToken     string
	protocolVersion semver.Version
	maxTries        int
}

func getDefaultConfiguration(baseURL string) clientConfiguration {
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
		panic("64-bit architecture required")
	}

	configuration := getDefaultConfiguration(baseURL)

	for _, applyOption := range options {
		if err := applyOption(&configuration); err != nil {
			return Client{}, err
		}
	}

	client := Client{configuration}

	return client, nil
}
