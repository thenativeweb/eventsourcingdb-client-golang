package event

import (
	"encoding/json"
	"time"
)

const TimeFormat = time.RFC3339Nano

type Timestamp struct {
	time.Time
}

func NewTimestamp(time time.Time) Timestamp {
	return Timestamp{Time: time.UTC()}
}

func (timestamp Timestamp) MarshalJSON() ([]byte, error) {
	ts := timestamp.Time.Format(TimeFormat)
	return json.Marshal(ts)
}

func (timestamp *Timestamp) UnmarshalJSON(data []byte) error {
	var timeString string
	if err := json.Unmarshal(data, &timeString); err != nil {
		return err
	}

	// Parse the string as a time value in the RFC3339 format with nanoseconds
	parsedTime, err := time.Parse(TimeFormat, timeString)
	if err != nil {
		return err
	}

	timestamp.Time = parsedTime

	return nil
}
