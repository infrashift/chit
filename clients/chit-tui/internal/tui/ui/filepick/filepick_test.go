package filepick_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/filepick"
)

// openIn opens a picker in dir and feeds it the result of the read.
func openIn(t *testing.T, dir string, maxBytes int64) filepick.Model {
	t.Helper()
	m := filepick.New(testutil.Styles(), "Attach a file", []string{".png"}, maxBytes)
	m.SetSize(100, 40)
	cmd := m.Open(dir)
	if !m.Visible() {
		t.Fatal("Open did not open")
	}
	m, _ = m.Update(cmd())
	return m
}

func write(t *testing.T, dir, name string, size int) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
}

func enter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

// pickFirst presses Enter on the first entry and returns the messages of
// the command.
func pickFirst(t *testing.T, m filepick.Model) (filepick.Model, []tea.Msg) {
	t.Helper()
	m, cmd := m.Update(enter())
	return m, messagesOf(cmd)
}

// messagesOf runs cmd and returns its messages, one level of batch deep.
func messagesOf(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		return []tea.Msg{cmd()}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		if c != nil {
			msgs = append(msgs, c())
		}
	}
	return msgs
}

func TestFilepick_PicksAnAllowedFile(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.png", 10)
	m := openIn(t, dir, 100)
	if v := testutil.StripANSI(m.View()); !strings.Contains(v, "a.png") || !strings.Contains(v, filepath.Base(dir)) {
		t.Fatalf("View = %q", v)
	}
	m, msgs := pickFirst(t, m)
	var picked *filepick.PickedMsg
	for _, msg := range msgs {
		if p, ok := msg.(filepick.PickedMsg); ok {
			picked = &p
		}
	}
	if picked == nil || picked.Path != filepath.Join(dir, "a.png") || picked.Size != 10 {
		t.Fatalf("messages = %#v, want a PickedMsg for a.png", msgs)
	}
	if m.Visible() {
		t.Error("the picker stayed open after a pick")
	}
}

func TestFilepick_RefusesALargeFile(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "big.png", 200)
	m, msgs := pickFirst(t, openIn(t, dir, 100))
	for _, msg := range msgs {
		if _, ok := msg.(filepick.PickedMsg); ok {
			t.Fatal("picked a file over the cap")
		}
	}
	if !m.Visible() || !strings.Contains(testutil.StripANSI(m.View()), "too large") {
		t.Errorf("no reason shown: %q", testutil.StripANSI(m.View()))
	}
}

func TestFilepick_RefusesAnotherType(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "notes.txt", 1)
	m, msgs := pickFirst(t, openIn(t, dir, 100))
	for _, msg := range msgs {
		if _, ok := msg.(filepick.PickedMsg); ok {
			t.Fatal("picked a file of another type")
		}
	}
	if !m.Visible() || !strings.Contains(testutil.StripANSI(m.View()), "pick a file of type .png") {
		t.Errorf("no reason shown: %q", testutil.StripANSI(m.View()))
	}
}

// A long directory keeps its end on one line, not wrapped in the frame.
func TestFilepick_LongDirectoryKeepsItsEnd(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("deep", 20), "photos")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(testutil.StripANSI(openIn(t, dir, 100).View()), "\n")
	found := false
	for _, l := range lines {
		found = found || (strings.Contains(l, "…") && strings.Contains(l, "photos"))
	}
	if !found {
		t.Errorf("no line holds the cut directory:\n%s", strings.Join(lines, "\n"))
	}
}

func TestFilepick_EscCancels(t *testing.T) {
	m := openIn(t, t.TempDir(), 100)
	m.SetStyles(testutil.Styles())
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if _, ok := cmd().(filepick.CancelledMsg); !ok || m.Visible() {
		t.Errorf("Esc: %#v, visible %v", cmd(), m.Visible())
	}
	if m.View() != "" {
		t.Error("a closed picker drew something")
	}
	if _, cmd := m.Update(enter()); cmd != nil {
		t.Error("a closed picker took a key")
	}
}
