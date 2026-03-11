package model

import "time"

// MillisToTime converts Unix milliseconds to time.Time.
func MillisToTime(millis int64) time.Time {
	return time.UnixMilli(millis)
}
