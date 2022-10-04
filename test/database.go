package test

import (
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
)

type WithAuthorization struct {
	Container   docker.Container
	BaseURL     string
	AccessToken string
	Client      eventsourcingdb.Client
}

type WithoutAuthorization struct {
	Container docker.Container
	BaseURL   string
	Client    eventsourcingdb.Client
}

type WithInvalidURL struct {
	BaseURL string
	Client  eventsourcingdb.Client
}

type Database struct {
	WithAuthorization    WithAuthorization
	WithoutAuthorization WithoutAuthorization
	WithInvalidURL       WithInvalidURL
}
