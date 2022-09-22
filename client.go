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

type ClientOptions struct {
	Timeout         time.Duration
	AccessToken     string
	ProtocolVersion string
}

func GetDefaultClientOptions() ClientOptions {
	return ClientOptions{
		Timeout:         10 * time.Second,
		AccessToken:     "",
		ProtocolVersion: "1.0.0",
	}
}

type Client struct {
	configuration ClientConfiguration
}

func NewClient(baseUrl string, options ClientOptions) Client {
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
		configuration.protocolVersion = *semver.MustParse(options.AccessToken)
	}

	client := Client{configuration}

	return client
}
