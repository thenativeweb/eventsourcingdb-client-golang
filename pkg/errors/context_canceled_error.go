package errors

import "context"

type ContextCanceledError struct {
	ctx context.Context
}

func (error *ContextCanceledError) Error() string {
	return error.ctx.Err().Error()
}

func NewContextCanceledError(ctx context.Context) error {
	return &ContextCanceledError{ctx}
}

func IsContextCanceledError(err error) bool {
	_, ok := err.(*ContextCanceledError)

	return ok
}
