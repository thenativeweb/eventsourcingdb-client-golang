package test

import (
	"path"

	"github.com/google/uuid"
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
)

func Setup() (Database, error) {
	dockerfilePath := path.Join("docker", "eventsourcingdb")
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
		eventsourcingdb.ClientOptions{AccessToken: accessToken},
	)
	if err != nil {
		return Database{}, err
	}

	withoutAuthorization, err := NewContainerizedTestingDatabase(
		image,
		[]string{"server", "--dev", "--ui"},
		eventsourcingdb.ClientOptions{},
	)
	if err != nil {
		return Database{}, err
	}

	withInvalidURL := NewTestingDatabase(eventsourcingdb.NewClient("http://localhost.invalid"))

	database := Database{
		withAuthorization,
		withoutAuthorization,
		withInvalidURL,
	}

	return database, nil
}
