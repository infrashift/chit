package model

import (
	"time"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// MillisToTime converts Unix milliseconds to time.Time.
func MillisToTime(millis int64) time.Time {
	t := time.UnixMilli(millis)
	assert.That(t.UnixMilli() == millis, "MillisToTime: conversion must round-trip")
	return t
}
