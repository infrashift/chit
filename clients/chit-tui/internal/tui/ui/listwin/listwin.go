// Package listwin decides which rows of a list an overlay draws, so the
// cursor stays on screen however long the list is. Every picker drew from
// the first row and capped the count, so moving the cursor past the cap
// moved it out of sight.
package listwin

import "github.com/infrashift/chit/clients/chit-tui/internal/assert"

// Rows is the row budget for a list in an overlay on a terminal this tall,
// leaving room for the overlay's frame and header.
func Rows(height int) int {
	rows := max(height/2-4, 5)
	assert.That(rows >= 5, "listwin.Rows: budget below the minimum of 5")
	return rows
}

// Window returns the half-open range [start, end) of rows to draw: at most
// visible rows out of total, scrolled just far enough to show cursor.
func Window(cursor, total, visible int) (start, end int) {
	if visible <= 0 || total <= 0 {
		return 0, 0
	}
	start = Start(cursor, total, visible)
	end = min(start+visible, total)
	assert.That(start >= 0 && start <= end, "listwin.Window: start outside [0, end]")
	assert.That(end <= total, "listwin.Window: end past the list")
	assert.That(end-start <= visible, "listwin.Window: window wider than visible")
	assert.That(cursor < 0 || cursor >= total || (cursor >= start && cursor < end), "listwin.Window: cursor outside the window")
	return start, end
}

// Start returns the first row Window draws, for a caller that needs only
// where the window begins.
func Start(cursor, total, visible int) int {
	if visible <= 0 || total <= 0 {
		return 0
	}
	start := 0
	if cursor >= visible {
		start = cursor - visible + 1
	}
	start = min(start, max(total-visible, 0))
	assert.That(start >= 0, "listwin.Start: negative start")
	assert.That(start < total, "listwin.Start: start past the last row")
	assert.That(cursor < 0 || cursor >= total || cursor >= start, "listwin.Start: window starts below the cursor")
	return start
}
