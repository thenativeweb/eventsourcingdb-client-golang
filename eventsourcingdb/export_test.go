package eventsourcingdb

import (
	"testing"
	"time"
)

// SetHeartbeatTimeout shortens the heartbeat timeout until the test t ends,
// so that it does not have to wait for 30 seconds.
func SetHeartbeatTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()

	original := heartbeatTimeout
	heartbeatTimeout = timeout

	t.Cleanup(func() {
		heartbeatTimeout = original
	})
}
