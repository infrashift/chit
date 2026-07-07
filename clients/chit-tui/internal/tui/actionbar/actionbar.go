// Package actionbar renders the bottom action/status bar: clickable action
// buttons, the active team/channel context, transient errors, and the
// connection indicator. Every button mirrors an existing keybinding so the
// TUI remains fully keyboard-drivable.
package actionbar

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// Action identifies a clickable action on the bar.
type Action int

const (
	ActionNone Action = iota
	ActionPalette
	ActionSearch
	ActionPeople
	ActionNewChannel
	ActionHelp
	ActionCloseThread
)

type button struct {
	label  string
	action Action
}

// span is the half-open cell range [start, end) a button occupies on screen.
type span struct {
	start  int
	end    int
	action Action
}

// Model is the action bar component.
type Model struct {
	width      int
	team       string
	channel    string
	user       string
	err        string
	threadOpen bool
	connected  bool
	styles     styles.Styles
}

// New creates a new action bar.
func New(s styles.Styles) Model {
	return Model{styles: s}
}

// SetSize sets the bar width.
func (m *Model) SetSize(w int) { m.width = w }

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetContext sets the team, channel, and user names shown on the bar.
func (m *Model) SetContext(team, channel, user string) {
	m.team = team
	m.channel = channel
	m.user = user
}

// SetThreadOpen toggles the thread-context button set.
func (m *Model) SetThreadOpen(open bool) { m.threadOpen = open }

// SetConnected sets the WebSocket connection indicator state.
func (m *Model) SetConnected(c bool) { m.connected = c }

// SetError sets the transient error text (empty clears it).
func (m *Model) SetError(e string) { m.err = e }

func (m Model) buttons() []button {
	btns := make([]button, 0, 5)
	if m.threadOpen {
		btns = append(btns, button{label: "esc Back", action: ActionCloseThread})
	}
	btns = append(btns,
		button{label: "^K Jump", action: ActionPalette},
		button{label: "^S Search", action: ActionSearch},
		button{label: "^D DM", action: ActionPeople},
		button{label: "^N New", action: ActionNewChannel},
		button{label: "? Help", action: ActionHelp},
	)
	return btns
}

// layout renders the bar and records the clickable spans as it goes. It is
// the single source of truth for button geometry: View and HitTest both use
// it so clicks can never drift from what is drawn.
func (m Model) layout() (string, []span) {
	base := m.styles.StatusBar.Padding(0, 0)

	var b strings.Builder
	var spans []span
	x := 0
	write := func(seg string) {
		b.WriteString(seg)
		x += ansi.StringWidth(seg)
	}

	write(base.Render(" "))
	for i, btn := range m.buttons() {
		if i > 0 {
			write(base.Render(" "))
		}
		start := x
		write(m.styles.BarButton.Render("[" + btn.label + "]"))
		spans = append(spans, span{start: start, end: x, action: btn.action})
	}

	ctx := " │ " + m.team
	if m.channel != "" {
		ctx += " > " + m.channel
	}
	write(base.Render(ctx))

	if m.err != "" {
		write(m.styles.ErrorText.Render(" " + m.err))
	}

	dot := m.styles.UnreadBadge.Render("●")
	if !m.connected {
		dot = m.styles.ErrorText.Render("○")
	}
	right := dot + base.Render(" "+m.user+" ")

	gap := m.width - x - ansi.StringWidth(right)
	if gap < 0 {
		gap = 0
	}
	write(base.Render(strings.Repeat(" ", gap)))
	write(right)

	return b.String(), spans
}

// View renders the action bar as a single line.
func (m Model) View() string {
	s, _ := m.layout()
	return s
}

// HitTest returns the action under screen column x, or ActionNone.
func (m Model) HitTest(x int) Action {
	_, spans := m.layout()
	for _, sp := range spans {
		if x >= sp.start && x < sp.end {
			return sp.action
		}
	}
	return ActionNone
}
