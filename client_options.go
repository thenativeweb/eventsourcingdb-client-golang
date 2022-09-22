package eventsourcingdb

import "time"

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
