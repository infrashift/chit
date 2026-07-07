package viewport

import (
	"strings"
	"testing"
	"time"
)

func TestSameDay_True(t *testing.T) {
	a := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	b := time.Date(2026, 3, 10, 23, 59, 59, 0, time.UTC)
	if !sameDay(a, b) {
		t.Error("expected same day")
	}
}

func TestSameDay_False(t *testing.T) {
	a := time.Date(2026, 3, 10, 23, 59, 59, 0, time.UTC)
	b := time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC)
	if sameDay(a, b) {
		t.Error("expected different days")
	}
}

func TestFormatDaySeparator(t *testing.T) {
	ts := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	result := formatDaySeparator(ts, 60)

	if !strings.Contains(result, "Tuesday") {
		t.Errorf("expected day name 'Tuesday' in %q", result)
	}
	if !strings.Contains(result, "2026-03-10") {
		t.Errorf("expected date in %q", result)
	}
	if !strings.Contains(result, "─") {
		t.Errorf("expected separator chars in %q", result)
	}
	// Total display width should match requested width.
	if len([]rune(result)) != 60 {
		t.Errorf("expected rune len 60, got %d: %q", len([]rune(result)), result)
	}
}

func TestFormatDaySeparator_NarrowWidth(t *testing.T) {
	ts := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	// Width smaller than the label — should degrade gracefully (just the label).
	result := formatDaySeparator(ts, 5)

	if !strings.Contains(result, "Tuesday") {
		t.Errorf("expected day name in narrow result %q", result)
	}
	// Should not contain separator dashes since there's no room.
	if strings.HasPrefix(result, "─") {
		t.Errorf("expected no leading dashes in narrow result %q", result)
	}
}
