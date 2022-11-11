package test

import (
	"github.com/thenativeweb/eventsourcingdb-client-golang"
)

type TestingDatabase struct {
	client eventsourcingdb.Client
}

func (database TestingDatabase) GetClient() eventsourcingdb.Client {
	return database.client
}

func NewTestingDatabase(client eventsourcingdb.Client) TestingDatabase {
	return TestingDatabase{
		client: client,
	}
}
