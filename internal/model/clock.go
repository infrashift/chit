package model

import (
	"time"

	"github.com/infrashift/chit/internal/assert"
)

// GetMillis returns the current time in milliseconds since the Unix epoch.
func GetMillis() int64 {
	ms := time.Now().UnixMilli()
	assert.That(ms > 0, "GetMillis: clock before the epoch")
	return ms
}

// MillisToTime converts milliseconds since the Unix epoch to a time.Time.
func MillisToTime(millis int64) time.Time {
	t := time.UnixMilli(millis)
	assert.That(t.UnixMilli() == millis, "MillisToTime: round trip lost precision")
	return t
}
