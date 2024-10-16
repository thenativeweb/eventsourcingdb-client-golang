package test

import (
	"errors"
	"strconv"

	"github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/docker"
)

type ContainerizedTestingDatabase struct {
	TestingDatabase

	image       docker.Image
	command     []string
	accessToken string

	container docker.Container

	isFirstRun bool
}

func NewContainerizedTestingDatabase(image docker.Image, command []string, accessToken string) (ContainerizedTestingDatabase, error) {
	database := ContainerizedTestingDatabase{
		TestingDatabase: TestingDatabase{},
		image:           image,
		command:         command,
		accessToken:     accessToken,
		container:       docker.Container{},
		isFirstRun:      true,
	}

	startResult, err := database.start()
	if err != nil {
		return database, err
	}

	database.container = startResult.container
	database.client = startResult.client

	var retryError error
	for try := 0; try < 10; try++ {
		err := database.client.Ping()
		if err != nil {
			retryError = errors.Join(retryError, err)
			continue
		}

		retryError = nil
		break
	}
	if retryError != nil {
		return ContainerizedTestingDatabase{}, retryError
	}

	return database, nil
}

func (database *ContainerizedTestingDatabase) GetClient() eventsourcingdb.Client {
	if database.isFirstRun {
		database.isFirstRun = false

		return database.client
	}

	database.restart()

	return database.client
}

type startResult struct {
	client    eventsourcingdb.Client
	container docker.Container
}

func (database *ContainerizedTestingDatabase) start() (startResult, error) {
	container, err := database.image.Run(database.command, true, true)
	if err != nil {
		return startResult{}, err
	}

	port, err := container.GetExposedPort(3000)
	if err != nil {
		return startResult{}, err
	}

	baseURL := "http://localhost:" + strconv.Itoa(port)
	client, err := eventsourcingdb.NewClient(baseURL, database.accessToken)
	if err != nil {
		return startResult{}, err
	}

	var retryError error
	for try := 0; try < 10; try++ {
		err := client.Ping()
		if err != nil {
			retryError = errors.Join(retryError, err)
			continue
		}

		retryError = nil
		break
	}
	if retryError != nil {
		return startResult{}, retryError
	}

	return startResult{
		container: container,
		client:    client,
	}, nil
}

func (database *ContainerizedTestingDatabase) restart() {
	if err := database.container.Kill(); err != nil {
		panic("could not kill database container")
	}

	startResult, err := database.start()
	if err != nil {
		panic("could not restart database container")
	}

	database.client = startResult.client
	database.container = startResult.container
}

func (database *ContainerizedTestingDatabase) Stop() error {
	if err := database.container.Kill(); err != nil {
		return err
	}

	return nil
}
