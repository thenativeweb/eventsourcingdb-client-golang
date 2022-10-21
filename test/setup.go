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

	accessToken := uuid.New().String()
	container, baseURL, client, err := runDatabase(func() (docker.Container, error) {
		return docker.RunContainer(image, "server", true, true, "--dev", "--ui", "--access-token", accessToken)
	}, eventsourcingdb.ClientOptions{
		AccessToken: accessToken,
	})
	if err != nil {
		return Database{}, err
	}
	withAuthorization := WithAuthorization{container, baseURL, accessToken, client}

	container, baseURL, client, err = runDatabase(func() (docker.Container, error) {
		return docker.RunContainer(image, "server", true, true, "--dev", "--ui")
	}, eventsourcingdb.ClientOptions{})
	if err != nil {
		return Database{}, err
	}
	withoutAuthorization := WithoutAuthorization{container, baseURL, client}

	baseURL = "http://localhost.invalid"
	client = eventsourcingdb.NewClient(baseURL)
	withInvalidURL := WithInvalidURL{baseURL, client}

	database := Database{
		withAuthorization,
		withoutAuthorization,
		withInvalidURL,
	}

	return database, nil
}
