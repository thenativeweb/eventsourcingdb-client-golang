package configuration

import (
	"net/url"

	"github.com/Masterminds/semver"
)

type ClientConfiguration struct {
	BaseURL         *url.URL
	AccessToken     string
	ProtocolVersion semver.Version
	MaxTries        int
}

func GetDefaultConfiguration(baseURL *url.URL, accessToken string) ClientConfiguration {
	return ClientConfiguration{
		BaseURL:         baseURL,
		AccessToken:     accessToken,
		ProtocolVersion: *semver.MustParse("1.0.0"),
		MaxTries:        10,
	}
}
