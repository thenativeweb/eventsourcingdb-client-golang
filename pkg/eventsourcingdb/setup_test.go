package eventsourcingdb_test

import (
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test"
	"log"
	"os"
	"path"
	"testing"
)

var database test.Database

func TestMain(m *testing.M) {
	var err error

	database, err = test.Setup(path.Join("..", "..", "internal", "test", "docker", "eventsourcingdb"))
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	exitCode := m.Run()

	err = test.Teardown(database)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}
