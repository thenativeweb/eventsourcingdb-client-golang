package eventsourcingdb

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver"
	"time"
)

type ClientOption func(configuration *clientConfiguration) error

func ClientWithMaxTries(maxTries int) ClientOption {
	return func(configuration *clientConfiguration) error {
		if maxTries < 1 {
			return errors.New("maxTries must be 1 or greater")
		}

		configuration.maxTries = maxTries

		return nil
	}
}

func ClientWithTimeout(timeout time.Duration) ClientOption {
	return func(configuration *clientConfiguration) error {
		configuration.timeout = timeout

		return nil
	}
}

func ClientWithAccessToken(accessToken string) ClientOption {
	return func(configuration *clientConfiguration) error {
		if accessToken == "" {
			return errors.New("the access token must not be empty")
		}

		configuration.accessToken = accessToken

		return nil
	}
}

func ClientWithProtocolVersion(protocolVersion string) ClientOption {
	return func(configuration *clientConfiguration) error {
		version, err := semver.NewVersion(protocolVersion)
		if err != nil {
			return fmt.Errorf("the protocol version must be a valid semver version: %w", err)
		}

		configuration.protocolVersion = *version

		return nil
	}
}
