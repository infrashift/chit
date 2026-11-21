package infobox_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/infobox"
)

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestInfobox_ShowsRowsAndHints(t *testing.T) {
	m := infobox.New(testutil.Styles())
	m.SetSize(120, 40)
	if m.Visible() || m.View() != "" {
		t.Fatal("a new box is open")
	}
	m.Open("post:p1", "Post", []infobox.Row{{"Author", "@alice"}, {"Model", "claude"}}, []infobox.Action{{Key: "s", Label: "save files"}})
	v := testutil.StripANSI(m.View())
	for _, want := range []string{"Post", "Author   @alice", "Model    claude", "s save files · esc close"} {
		if !strings.Contains(v, want) {
			t.Errorf("View lacks %q:\n%s", want, v)
		}
	}
	if m.Subject() != "post:p1" {
		t.Errorf("Subject = %q", m.Subject())
	}
}

func TestInfobox_ActionAndClose(t *testing.T) {
	m := infobox.New(testutil.Styles())
	m.Open("post:p1", "Post", nil, []infobox.Action{{Key: "s", Label: "save"}})
	m, cmd := m.Update(runes("s"))
	if msg, ok := cmd().(infobox.ActionMsg); !ok || msg.Key != "s" || msg.Subject != "post:p1" || !m.Visible() {
		t.Errorf("s sent %#v, visible %v", cmd(), m.Visible())
	}
	if _, cmd = m.Update(runes("x")); cmd != nil {
		t.Error("a key with no action sent a message")
	}
	m, cmd = m.Update(runes("q"))
	if msg, ok := cmd().(infobox.ClosedMsg); !ok || msg.Subject != "post:p1" || m.Visible() {
		t.Errorf("q sent %#v, visible %v", cmd(), m.Visible())
	}
	if _, cmd = m.Update(runes("s")); cmd != nil {
		t.Error("a closed box took a key")
	}
	m.Open("x", "X", nil, nil)
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if _, ok := cmd().(infobox.ClosedMsg); !ok || m.Visible() {
		t.Error("Esc did not close the box")
	}
}

func TestInfobox_ScrollsALongList(t *testing.T) {
	m := infobox.New(testutil.Styles())
	m.SetSize(120, 20)
	m.SetStyles(testutil.Styles())
	var rows []infobox.Row
	for i := range 30 {
		rows = append(rows, infobox.Row{Label: "tool", Value: fmt.Sprintf("t%02d", i)})
	}
	m.Open("s", "Long", rows, nil)
	if v := testutil.StripANSI(m.View()); !strings.Contains(v, "t00") || strings.Contains(v, "t29") || !strings.Contains(v, "scroll") {
		t.Fatalf("first page:\n%s", v)
	}
	for range 40 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if v := testutil.StripANSI(m.View()); !strings.Contains(v, "t29") || strings.Contains(v, "t00") {
		t.Errorf("last page:\n%s", v)
	}
	for range 40 {
		m, _ = m.Update(runes("k"))
	}
	if v := testutil.StripANSI(m.View()); !strings.Contains(v, "t00") {
		t.Errorf("back at the top:\n%s", v)
	}
}

func TestInfobox_CapsTheRows(t *testing.T) {
	m := infobox.New(testutil.Styles())
	m.SetSize(120, 1000)
	rows := make([]infobox.Row, 500)
	for i := range rows {
		rows[i] = infobox.Row{Label: "r", Value: fmt.Sprint(i)}
	}
	m.Open("s", "Many", rows, nil)
	if v := testutil.StripANSI(m.View()); strings.Contains(v, " 200 ") || !strings.Contains(v, " 199") {
		t.Errorf("the box did not stop at 200 rows")
	}
}
