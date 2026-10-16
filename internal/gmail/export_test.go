package gmail

import "time"

// SetSyncTimeout shortens one mailbox's sync deadline for a test, and returns a func to put it back.
func SetSyncTimeout(d time.Duration) func() {
	before := syncTimeout
	syncTimeout = d
	return func() { syncTimeout = before }
}
