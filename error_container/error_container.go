package error_container

type ErrorContainer struct {
	Error error
}

func (err ErrorContainer) IsError() bool {
	return err.Error != nil
}

func (err ErrorContainer) IsOkay() bool {
	return !err.IsError()
}

func NewErrorContainer(err error) ErrorContainer {
	return ErrorContainer{Error: err}
}
