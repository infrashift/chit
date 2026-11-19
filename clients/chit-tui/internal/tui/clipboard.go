package tui

import (
	"fmt"
	"io"

	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/infrashift/chit/clients/chit-tui/internal/errlog"
)

// copyToClipboard writes text to the system clipboard using OSC 52, sending
// the sequence to out (the terminal; the Model's clipboardOut).
//
// OSC 52 is an escape sequence the terminal itself acts on, which is what
// makes it work over SSH and inside multiplexers where a local clipboard API
// would only reach the remote machine. Support is not universal — Terminal.app
// ignores it, and tmux and screen need it enabled — so a copy can silently do
// nothing, which is why the UI confirms it rather than staying quiet.
func copyToClipboard(out io.Writer, text string) tea.Cmd {
	return func() tea.Msg {
		if text == "" {
			return nil
		}
		// Written straight to the terminal rather than through the renderer:
		// this is a control sequence, not content, and must not be laid out.
		// A failed write means the terminal did not receive the sequence;
		// there is no recovery and no channel to report it on, and the
		// acknowledgement below is already best-effort for the same reason.
		if _, err := osc52.New(text).WriteTo(out); err != nil {
			errlog.Debug(err, "write OSC 52 clipboard sequence")
		}
		return clipboardCopiedMsg{lines: countLines(text)}
	}
}

// clipboardCopiedMsg reports a completed copy so the UI can acknowledge it.
type clipboardCopiedMsg struct{ lines int }

func countLines(s string) int {
	n := 1
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}

func (m Model) handleClipboardCopied(msg clipboardCopiedMsg) (tea.Model, tea.Cmd) {
	// OSC 52 gives no delivery confirmation, and several terminals ignore
	// it outright, so say what was sent rather than leaving the user to
	// guess whether anything happened.
	noun := "lines"
	if msg.lines == 1 {
		noun = "line"
	}
	m.viewport.ClearSelection()
	return m, m.setError(fmt.Errorf("copied %d %s to the clipboard", msg.lines, noun))
}
