package eventsourcingdb

import (
	"time"

	"github.com/Masterminds/semver"
)

type ClientConfiguration struct {
	baseUrl         string
	timeout         time.Duration
	accessToken     string
	protocolVersion semver.Version
}

type Client struct {
	configuration ClientConfiguration
}

func NewClientWithOptions(baseUrl string, options ClientOptions) Client {
	defaultOptions := GetDefaultClientOptions()
	configuration := ClientConfiguration{
		baseUrl:         baseUrl,
		timeout:         defaultOptions.Timeout,
		accessToken:     defaultOptions.AccessToken,
		protocolVersion: *semver.MustParse(defaultOptions.ProtocolVersion),
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

func NewClient(baseUrl string) Client {
	return NewClientWithOptions(baseUrl, GetDefaultClientOptions())
}
