package timestamp

import (
	"encoding/json"
	"time"
)

type Timestamp struct {
	time.Time
}

func (timestamp *Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(timestamp.Time.Unix())
}

func (timestamp *Timestamp) UnmarshalJSON(data []byte) error {
	var unixSeconds int64

	if err := json.Unmarshal(data, &unixSeconds); err != nil {
		return err
	}

	timestamp.Time = time.Unix(unixSeconds, 0)

	return nil
}
