package tui

import (
	"os"

	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// copyToClipboard writes text to the system clipboard using OSC 52.
//
// OSC 52 is an escape sequence the terminal itself acts on, which is what
// makes it work over SSH and inside multiplexers where a local clipboard API
// would only reach the remote machine. Support is not universal — Terminal.app
// ignores it, and tmux and screen need it enabled — so a copy can silently do
// nothing, which is why the UI confirms it rather than staying quiet.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		if text == "" {
			return nil
		}
		// Written straight to the terminal rather than through the renderer:
		// this is a control sequence, not content, and must not be laid out.
		// A failed write means the terminal did not receive the sequence;
		// there is no recovery and no channel to report it on, and the
		// acknowledgement below is already best-effort for the same reason.
		_, _ = osc52.New(text).WriteTo(os.Stdout)
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
