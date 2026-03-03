package model

import "time"

// GetMillis returns the current time in milliseconds since the Unix epoch.
func GetMillis() int64 {
	return time.Now().UnixMilli()
}

// MillisToTime converts milliseconds since the Unix epoch to a time.Time.
func MillisToTime(millis int64) time.Time {
	return time.UnixMilli(millis)
}
