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
	ActionReply
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
	canReply   bool
	threadOpen bool
	editing    bool
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

// SetCanReply controls the Reply button. Replying requires a post to be
// selected in the history pane, and that pane has to be focused first — the
// button is the only on-screen hint that the capability exists at all.
func (m *Model) SetCanReply(can bool) { m.canReply = can }

// SetEditing marks a post edit in progress. Sending replaces that post
// rather than posting a new one, which nothing else on screen shows.
func (m *Model) SetEditing(e bool) { m.editing = e }

// SetConnected sets the WebSocket connection indicator state.
func (m *Model) SetConnected(c bool) { m.connected = c }

// SetError sets the transient error text (empty clears it).
func (m *Model) SetError(e string) { m.err = e }

func (m Model) buttons() []button {
	btns := make([]button, 0, 5)
	if m.threadOpen {
		btns = append(btns, button{label: "esc Back", action: ActionCloseThread})
	}
	if m.canReply {
		btns = append(btns, button{label: "↵ Reply", action: ActionReply})
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

	if m.editing {
		write(m.styles.MentionBadge.Render(" editing — enter saves, esc cancels "))
	}

	dot := m.styles.UnreadBadge.Render("●")
	if !m.connected {
		dot = m.styles.ErrorText.Render("○")
	}
	right := dot + base.Render(" "+m.user+" ")

	// What is left between the buttons and the connection dot holds the
	// team and channel, then any status message. A message that will not
	// fit beside them takes their room: the channel is on screen already,
	// the message is news. Whatever still does not fit is cut short, so the
	// bar never runs past the edge and pushes the dot and name off it.
	room := max(m.width-x-ansi.StringWidth(right), 0)
	ctx := " │ " + m.team
	if m.channel != "" {
		ctx += " > " + m.channel
	}
	status := ""
	if m.err != "" {
		status = " " + m.err
		if ansi.StringWidth(ctx)+ansi.StringWidth(status) > room {
			ctx = ""
		}
	}
	ctx = ansi.Truncate(ctx, room, "…")
	status = ansi.Truncate(status, room-ansi.StringWidth(ctx), "…")
	write(base.Render(ctx))
	if status != "" {
		write(m.styles.ErrorText.Render(status))
	}

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
