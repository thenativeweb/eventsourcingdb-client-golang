package test

import (
	"github.com/google/uuid"
	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/docker"
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
		[]string{"run", "--ui", "--access-token", accessToken, "--store-temporary"},
		accessToken,
	)
	if err != nil {
		return Database{}, err
	}

	client, err := eventsourcingdb.NewClient(
		"http://localhost.invalid",
		accessToken,
	)
	if err != nil {
		return Database{}, err
	}
	withInvalidURL := NewTestingDatabase(client)

	database := Database{
		withAuthorization,
		withInvalidURL,
	}

	return database, nil
}
