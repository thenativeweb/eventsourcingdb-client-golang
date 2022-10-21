package test

import (
	"github.com/thenativeweb/eventsourcingdb-client-golang"
	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
)

type TestingDatabase struct {
	clientOptions eventsourcingdb.ClientOptions
	client        eventsourcingdb.Client
	BaseURL       string
}

func NewTestingDatabase(clientOptions eventsourcingdb.ClientOptions, client eventsourcingdb.Client, baseURL string) TestingDatabase {
	return TestingDatabase{
		clientOptions: clientOptions,
		client:        client,
		BaseURL:       baseURL,
	}
}

func (database TestingDatabase) GetClient() eventsourcingdb.Client {
	return database.client
}

type ContainerizedTestingDatabase struct {
	TestingDatabase
	Container docker.Container
}

func NewContainerizedTestingDatabase(clientOptions eventsourcingdb.ClientOptions, client eventsourcingdb.Client, baseURL string, container docker.Container) ContainerizedTestingDatabase {
	return ContainerizedTestingDatabase{
		TestingDatabase{
			clientOptions: clientOptions,
			client:        client,
			BaseURL:       baseURL,
		},
		container,
	}
}

func (database *ContainerizedTestingDatabase) GetClient() eventsourcingdb.Client {
	newDatabase, err := database.restart()
	if err != nil {
		panic("could not restart database container")
	}

	*database = newDatabase

	return database.client
}
