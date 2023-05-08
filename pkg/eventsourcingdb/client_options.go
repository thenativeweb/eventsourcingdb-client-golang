package eventsourcingdb

import (
	"errors"
)

type ClientOption struct {
	apply func(configuration *clientConfiguration) error
	name  string
}

func MaxTries(maxTries int) ClientOption {
	return ClientOption{
		apply: func(configuration *clientConfiguration) error {
			if maxTries < 1 {
				return errors.New("maxTries must be 1 or greater")
			}

			configuration.maxTries = maxTries

			return nil
		},
		name: "MaxTries",
	}
}

func AccessToken(accessToken string) ClientOption {
	return ClientOption{
		apply: func(configuration *clientConfiguration) error {
			if accessToken == "" {
				return errors.New("the access token must not be empty")
			}

			configuration.accessToken = accessToken

			return nil
		},
		name: "AccessToken",
	}
}
