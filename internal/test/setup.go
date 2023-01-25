package test

import (
	"github.com/google/uuid"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/docker"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb"
	"time"
)

func Setup(dockerfilePath string) (Database, error) {
	image := docker.Image{
		Name: "eventsourcingdb",
		Tag:  "latest",
	}

	err := image.Build(dockerfilePath)
	if err != nil {
		return Database{}, err
	}

	accessToken := uuid.New().String()
	withAuthorization, err := NewContainerizedTestingDatabase(
		image,
		[]string{"server", "--dev", "--ui", "--access-token", accessToken},
		eventsourcingdb.ClientWithAccessToken(accessToken),
	)
	if err != nil {
		return Database{}, err
	}

	withoutAuthorization, err := NewContainerizedTestingDatabase(
		image,
		[]string{"server", "--dev", "--ui"},
	)
	if err != nil {
		return Database{}, err
	}

	client, err := eventsourcingdb.NewClient(
		"http://localhost.invalid",
		eventsourcingdb.ClientWithMaxTries(3),
		eventsourcingdb.ClientWithTimeout(1*time.Second),
	)
	if err != nil {
		return Database{}, err
	}
	withInvalidURL := NewTestingDatabase(client)

	database := Database{
		withAuthorization,
		withoutAuthorization,
		withInvalidURL,
	}

	return database, nil
}
