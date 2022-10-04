package eventsourcingdb_test

import (
	"log"
	"os"
	"testing"

	"github.com/thenativeweb/eventsourcingdb-client-golang/test"
)

var database test.Database

func TestMain(m *testing.M) {
	var err error

	database, err = test.Setup()
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
