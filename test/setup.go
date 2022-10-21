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
	clientOptions := eventsourcingdb.ClientOptions{
		AccessToken: accessToken,
	}
	container, baseURL, client, err := runDatabase(
		func() (docker.Container, error) {
			return image.Run("server", true, true, "--dev", "--ui", "--access-token", accessToken)
		},
		clientOptions,
	)
	if err != nil {
		return Database{}, err
	}
	withAuthorization := WithAuthorization{
		NewContainerizedTestingDatabase(clientOptions, client, baseURL, container),
		accessToken,
	}

	clientOptions = eventsourcingdb.ClientOptions{}
	container, baseURL, client, err = runDatabase(
		func() (docker.Container, error) {
			return image.Run("server", true, true, "--dev", "--ui")
		},
		clientOptions,
	)
	if err != nil {
		return Database{}, err
	}
	withoutAuthorization := WithoutAuthorization{
		NewContainerizedTestingDatabase(clientOptions, client, baseURL, container),
	}

	clientOptions = eventsourcingdb.ClientOptions{}
	baseURL = "http://localhost.invalid"
	client = eventsourcingdb.NewClient(baseURL)
	withInvalidURL := WithInvalidURL{
		NewTestingDatabase(clientOptions, client, baseURL),
	}

	database := Database{
		withAuthorization,
		withoutAuthorization,
		withInvalidURL,
	}

	return database, nil
}
