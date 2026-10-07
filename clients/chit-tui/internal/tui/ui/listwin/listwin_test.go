package listwin_test

import (
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/listwin"
)

func TestWindow(t *testing.T) {
	tests := []struct {
		name                   string
		cursor, total, visible int
		wantStart, wantEnd     int
	}{
		{"short list", 2, 3, 5, 0, 3},
		{"cursor in the first page", 4, 20, 5, 0, 5},
		{"cursor past the first page", 5, 20, 5, 1, 6},
		{"cursor on the last row", 19, 20, 5, 15, 20},
		{"empty", 0, 0, 5, 0, 0},
		{"cursor beyond the end", 30, 20, 5, 15, 20},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := listwin.Window(tc.cursor, tc.total, tc.visible)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Errorf("Window(%d, %d, %d) = [%d, %d), want [%d, %d)",
					tc.cursor, tc.total, tc.visible, start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}
