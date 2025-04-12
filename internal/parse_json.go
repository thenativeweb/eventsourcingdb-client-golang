package internal

import (
	"encoding/json"
	"io"
)

func ParseJSON(reader io.ReadCloser, v any) error {
	defer reader.Close()

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, v)
	if err != nil {
		return err
	}

	return nil
}
