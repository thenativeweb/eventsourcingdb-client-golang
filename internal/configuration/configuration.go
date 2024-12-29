package configuration

import (
	"net/url"

	"github.com/Masterminds/semver"
)

type ClientConfiguration struct {
	BaseURL         *url.URL
	APIToken        string
	ProtocolVersion semver.Version
}

func GetDefaultConfiguration(baseURL *url.URL, apiToken string) ClientConfiguration {
	return ClientConfiguration{
		BaseURL:         baseURL,
		APIToken:        apiToken,
		ProtocolVersion: *semver.MustParse("1.0.0"),
	}
}
