package test

import (
	"errors"
)

func Teardown(database Database) error {
	errorMessage := ""

	err := database.WithAuthorization.Stop()
	if err != nil {
		errorMessage = errorMessage + ": " + err.Error()
	}

	err = database.WithoutAuthorization.Stop()
	if err != nil {
		errorMessage = errorMessage + ": " + err.Error()
	}

	if errorMessage != "" {
		return errors.New(errorMessage)
	}

	return nil
}
