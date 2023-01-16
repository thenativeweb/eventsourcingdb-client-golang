package result

type Result[TData any] struct {
	error error
	data  TData
}

func (result Result[TData]) IsError() bool {
	return result.error != nil
}

func (result Result[TData]) IsData() bool {
	return !result.IsError()
}

func NewResultWithData[TData any](data TData) Result[TData] {
	return Result[TData]{error: nil, data: data}
}

func NewResultWithError[TData any](err error) Result[TData] {
	var nilData TData

	return Result[TData]{error: err, data: nilData}
}

func (result Result[TData]) GetData() (TData, error) {
	var nilData TData

	if result.IsError() {
		return nilData, result.error
	}

	return result.data, nil
}
