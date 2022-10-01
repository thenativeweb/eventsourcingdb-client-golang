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

	err := docker.BuildImage(dockerfilePath, image)
	if err != nil {
		return Database{}, err
	}

	database := Database{}

	accessToken := uuid.New().String()
	container, baseURL, client, err := runDatabase(func() (docker.Container, error) {
		return docker.RunContainer(image, "server", true, true, "--dev", "--access-token", accessToken)
	}, eventsourcingdb.ClientOptions{
		AccessToken: accessToken,
	})
	if err != nil {
		return database, err
	}
	database.WithAuthorization = WithAuthorization{container, baseURL, accessToken, client}

	container, baseURL, client, err = runDatabase(func() (docker.Container, error) {
		return docker.RunContainer(image, "server", true, true, "--dev")
	}, eventsourcingdb.ClientOptions{})
	if err != nil {
		return database, err
	}
	database.WithoutAuthorization = WithoutAuthorization{container, baseURL, client}

	return database, nil
}
