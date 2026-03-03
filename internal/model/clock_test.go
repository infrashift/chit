package model

import (
	"testing"
	"time"
)

func TestGetMillis(t *testing.T) {
	ms := GetMillis()
	if ms <= 0 {
		t.Fatal("GetMillis returned non-positive value")
	}
	now := time.Now().UnixMilli()
	diff := now - ms
	if diff < 0 {
		diff = -diff
	}
	if diff > 1000 {
		t.Fatalf("GetMillis (%d) differs from time.Now().UnixMilli() (%d) by more than 1s", ms, now)
	}
}

func TestMillisToTime(t *testing.T) {
	// Known value: 2024-01-01 00:00:00 UTC = 1704067200000 ms
	ms := int64(1704067200000)
	tm := MillisToTime(ms)
	expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if !tm.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, tm)
	}
}

func TestMillisToTime_Zero(t *testing.T) {
	tm := MillisToTime(0)
	if !tm.Equal(time.Unix(0, 0)) {
		t.Fatalf("expected Unix epoch, got %v", tm)
	}
}

func TestMillisRoundTrip(t *testing.T) {
	ms := GetMillis()
	tm := MillisToTime(ms)
	roundTrip := tm.UnixMilli()
	if ms != roundTrip {
		t.Fatalf("round-trip failed: %d -> %v -> %d", ms, tm, roundTrip)
	}
}
