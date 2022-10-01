package test

import (
	"errors"

	"github.com/thenativeweb/eventsourcingdb-client-golang/docker"
)

func Teardown(database Database) error {
	errorMessage := ""

	err := docker.KillContainer(database.WithAuthorization.Container)
	if err != nil {
		errorMessage = errorMessage + ": " + err.Error()
	}

	err = docker.KillContainer(database.WithoutAuthorization.Container)
	if err != nil {
		errorMessage = errorMessage + ": " + err.Error()
	}

	if errorMessage != "" {
		return errors.New(errorMessage)
	}

	return nil
}
