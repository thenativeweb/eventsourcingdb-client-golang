package eventsourcingdb

import (
	"encoding/json"
	"io"
)

// parseJSON reads all of the given reader, closes it, and decodes what it
// read as JSON into v.
func parseJSON(reader io.ReadCloser, v any) error {
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
