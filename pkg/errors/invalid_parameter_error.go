package errors

import "fmt"

type InvalidParameterError struct {
	parameterName string
	reason        string
}

func (err *InvalidParameterError) Error() string {
	return fmt.Sprintf("parameter '%s' is invalid: %s", err.parameterName, err.reason)
}

func NewInvalidParameterError(parameterName, reason string) error {
	return &InvalidParameterError{
		parameterName,
		reason,
	}
}

func IsInvalidParameterError(err error) bool {
	_, ok := err.(*InvalidParameterError)

	return ok
}
