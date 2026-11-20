package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/chcreator"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/help"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/threadinbox"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// modelWithEveryOverlayOpen returns a Model whose floating overlays are all
// visible, which never happens in use but lets a test see which one a key
// reached.
func modelWithEveryOverlayOpen() Model {
	s := styles.New(theme.TokyoNight())
	var m Model
	m.dmPicker = dmpicker.New(s)
	m.dmPicker.OpenForMembers()
	m.threadInbox = threadinbox.New(s)
	m.threadInbox.Open()
	m.skinPicker = skinpicker.New(s)
	m.skinPicker.Open()
	m.chCreator = chcreator.New(s)
	m.chCreator.Open("team-1")
	m.tagPicker = tagpicker.New(s)
	m.tagPicker.Open("post-1", nil, nil)
	m.palette = palette.New(s)
	m.palette.Open("")
	m.help = help.New(s)
	m.help.Open()
	return m
}

// updateOverlay must send the key to the overlay the ID names and to no
// other: Esc closes exactly that one, and its state lands in the Model.
func TestUpdateOverlayRoutesToTheNamedOverlay(t *testing.T) {
	esc := tea.KeyMsg{Type: tea.KeyEscape}
	ids := make(map[overlayID]bool)
	base := modelWithEveryOverlayOpen()
	for i, o := range base.overlays() {
		if ids[o.id] {
			t.Fatalf("overlay %d repeats id %d", i, o.id)
		}
		ids[o.id] = true

		m := modelWithEveryOverlayOpen()
		refs := m.overlays()
		for j, r := range refs {
			if !r.visible() {
				t.Fatalf("overlay %d did not open", j)
			}
		}
		m.updateOverlay(refs[i].id, esc)
		for j, r := range refs {
			if want := j != i; r.visible() != want {
				t.Errorf("after Esc to overlay %d: overlay %d visible = %v, want %v", i, j, r.visible(), want)
			}
		}
	}
	if len(ids) != int(overlayHelp)+1 {
		t.Errorf("overlays() lists %d overlays, want %d", len(ids), int(overlayHelp)+1)
	}
}
