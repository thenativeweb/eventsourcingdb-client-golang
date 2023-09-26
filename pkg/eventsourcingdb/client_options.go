package eventsourcingdb

import (
	"errors"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/configuration"
)

type ClientOption struct {
	apply func(configuration *configuration.ClientConfiguration) error
	name  string
}

func MaxTries(maxTries int) ClientOption {
	return ClientOption{
		apply: func(configuration *configuration.ClientConfiguration) error {
			if maxTries < 1 {
				return errors.New("maxTries must be 1 or greater")
			}

			configuration.MaxTries = maxTries

			return nil
		},
		name: "MaxTries",
	}
}
