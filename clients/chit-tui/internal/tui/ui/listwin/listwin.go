// Package listwin decides which rows of a list an overlay draws, so the
// cursor stays on screen however long the list is. Every picker drew from
// the first row and capped the count, so moving the cursor past the cap
// moved it out of sight.
package listwin

// Rows is the row budget for a list in an overlay on a terminal this tall,
// leaving room for the overlay's frame and header.
func Rows(height int) int { return max(height/2-4, 5) }

// Window returns the half-open range [start, end) of rows to draw: at most
// visible rows out of total, scrolled just far enough to show cursor.
func Window(cursor, total, visible int) (start, end int) {
	if visible <= 0 || total <= 0 {
		return 0, 0
	}
	if cursor >= visible {
		start = cursor - visible + 1
	}
	start = min(start, max(total-visible, 0))
	return start, min(start+visible, total)
}
