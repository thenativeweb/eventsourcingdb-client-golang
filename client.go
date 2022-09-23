package eventsourcingdb

import (
	"strconv"
	"time"

	"github.com/Masterminds/semver"
)

type ClientConfiguration struct {
	baseURL         string
	timeout         time.Duration
	accessToken     string
	protocolVersion semver.Version
	maxTries        int
}

type Client struct {
	configuration ClientConfiguration
}

func NewClientWithOptions(baseURL string, options ClientOptions) Client {
	if strconv.IntSize != 64 {
		panic("64-bit architecture required")
	}

	defaultOptions := GetDefaultClientOptions()
	configuration := ClientConfiguration{
		baseURL:         baseURL,
		timeout:         defaultOptions.Timeout,
		accessToken:     defaultOptions.AccessToken,
		protocolVersion: *semver.MustParse(defaultOptions.ProtocolVersion),
		maxTries:        10,
	}

	if options.Timeout != 0 {
		configuration.timeout = options.Timeout
	}
	if options.AccessToken != "" {
		configuration.accessToken = options.AccessToken
	}
	if options.ProtocolVersion != "" {
		configuration.protocolVersion = *semver.MustParse(options.ProtocolVersion)
	}

	client := Client{configuration}

	return client
}

func NewClient(baseURL string) Client {
	return NewClientWithOptions(baseURL, GetDefaultClientOptions())
}
