// Package threadinbox implements the overlay listing the threads the signed-in
// user follows: those in the active team, then those in direct and group
// channels. The two come from separate server lists, because a DM belongs to
// no team and so never appears in a team's.
package threadinbox

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// ThreadChosenMsg is sent when a thread is opened from the inbox.
type ThreadChosenMsg struct {
	// RootID identifies the thread; it is the root post's ID.
	RootID    string
	ChannelID string
}

// FollowToggledMsg is sent when the user follows or unfollows a thread.
type FollowToggledMsg struct {
	RootID    string
	Following bool
}

// ClosedMsg is sent when the overlay is dismissed.
type ClosedMsg struct{}

// Model is the thread-inbox overlay.
type Model struct {
	// threads is the team's threads followed by the direct ones; the cursor
	// moves over both as one list. teamCount is where the direct ones start.
	threads   []*model.ThreadResponse
	teamCount int
	teamName  string
	// channelNames labels each thread with where it lives; a root message on
	// its own rarely says which channel it is in.
	channelNames map[string]string
	cursor       int
	visible      bool
	focused      bool
	loading      bool
	styles       styles.Styles
	width        int
	height       int
}

// New creates a thread-inbox model.
func New(s styles.Styles) Model {
	return Model{styles: s, channelNames: map[string]string{}}
}

// Open shows the overlay in its loading state: the list is fetched when it
// opens, and an empty box would otherwise read as "no threads".
func (m *Model) Open() {
	m.visible = true
	m.focused = true
	m.loading = true
	m.cursor = 0
	m.threads = nil
}

// Close hides the overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.loading = false
}

// Visible reports whether the overlay is shown.
func (m Model) Visible() bool { return m.visible }

// Focus sets focus.
func (m *Model) Focus() { m.focused = true }

// Blur removes focus.
func (m *Model) Blur() { m.focused = false }

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetThreads replaces both lists, the active team's threads and those in
// direct and group channels, and ends the loading state.
func (m *Model) SetThreads(team, direct []*model.ThreadResponse) {
	m.threads = append(append([]*model.ThreadResponse{}, team...), direct...)
	m.teamCount = len(team)
	m.loading = false
	m.cursor = 0
}

// SetTeamName names the team whose threads head the list.
func (m *Model) SetTeamName(name string) { m.teamName = name }

// isDirect reports whether row i is in the direct-message section.
func (m Model) isDirect(i int) bool { return i >= m.teamCount }

// SetChannelNames supplies display names keyed by channel ID.
func (m *Model) SetChannelNames(names map[string]string) { m.channelNames = names }

// Threads returns the current list, for callers that need to look one up.
func (m Model) Threads() []*model.ThreadResponse { return m.threads }

// selected returns the thread under the cursor, or nil.
func (m Model) selected() *model.ThreadResponse {
	if m.cursor < 0 || m.cursor >= len(m.threads) {
		return nil
	}
	return m.threads[m.cursor]
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible || !m.focused {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "esc":
		m.Close()
		return m, func() tea.Msg { return ClosedMsg{} }
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < len(m.threads)-1 {
			m.cursor++
		}
		return m, nil
	case "u":
		// Unfollow drops the thread out of the list, so remove it here too
		// rather than leaving a row that no longer belongs.
		t := m.selected()
		if t == nil || t.Thread == nil {
			return m, nil
		}
		id := t.Thread.PostID
		if !m.isDirect(m.cursor) {
			m.teamCount--
		}
		m.threads = append(m.threads[:m.cursor], m.threads[m.cursor+1:]...)
		if m.cursor >= len(m.threads) {
			m.cursor = max(len(m.threads)-1, 0)
		}
		return m, func() tea.Msg { return FollowToggledMsg{RootID: id, Following: false} }
	case "enter":
		t := m.selected()
		if t == nil || t.Thread == nil {
			return m, nil
		}
		root, channel := t.Thread.PostID, t.Thread.ChannelID
		m.Close()
		return m, func() tea.Msg { return ThreadChosenMsg{RootID: root, ChannelID: channel} }
	}
	return m, nil
}

// unread reports whether a thread has replies the caller has not seen.
func unread(t *model.ThreadResponse) bool {
	if t == nil || t.Thread == nil {
		return false
	}
	return t.Thread.LastReplyAt > t.LastViewedAt
}

// View renders the overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	items := []string{m.styles.ListItemActive.Render("Threads you follow")}

	switch {
	case m.loading:
		items = append(items, "", m.styles.Timestamp.Render("  Loading…"))
	case len(m.threads) == 0:
		items = append(items, "",
			m.styles.Timestamp.Render("  You are not following any threads."),
			m.styles.Timestamp.Render("  Reply to a message to start following its thread."))
	default:
		for i, t := range m.threads {
			// A heading starts each non-empty section.
			if i == 0 && m.teamCount > 0 {
				items = append(items, "", m.styles.Timestamp.Render("  "+m.teamHeading()))
			}
			if i == m.teamCount {
				items = append(items, "", m.styles.Timestamp.Render("  Direct messages"))
			}
			items = append(items, m.renderRow(i, t))
		}
		items = append(items, "",
			m.styles.Timestamp.Render("  enter to open · u to unfollow · esc to close"))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(min(m.width*2/3, 78)).Render(body)
}

func (m Model) renderRow(i int, t *model.ThreadResponse) string {
	marker := "  "
	if unread(t) {
		marker = "● "
	}

	label := marker + m.channelLabel(i, t) + "  " + m.summary(t)
	if n := t.UnreadMentions; n > 0 {
		label += fmt.Sprintf("  (%d mention%s)", n, plural(n))
	}

	if i == m.cursor {
		return m.styles.ListItemActive.Render("> " + label)
	}
	return m.styles.ListItem.Render("  " + label)
}

func (m Model) teamHeading() string {
	if m.teamName == "" {
		return "In this team"
	}
	return "In " + m.teamName
}

// channelLabel says where row i's thread lives: #channel in the team section,
// and who the conversation is with in the direct section, where a # would
// suggest a channel that does not exist.
func (m Model) channelLabel(i int, t *model.ThreadResponse) string {
	prefix := "#"
	if m.isDirect(i) {
		prefix = ""
	}
	if t.Thread == nil {
		return prefix + "?"
	}
	if name, ok := m.channelNames[t.Thread.ChannelID]; ok && name != "" {
		return prefix + name
	}
	return prefix + "?"
}

// summary is the root message on one line. Without it the row identifies a
// thread only by its channel, which is no help when several are in the same
// one.
func (m Model) summary(t *model.ThreadResponse) string {
	root := t.Root()
	if root == nil || strings.TrimSpace(root.Content) == "" {
		return m.styles.Timestamp.Render("(no message)")
	}

	text := strings.Join(strings.Fields(root.Content), " ")
	const maxSummary = 48
	if len([]rune(text)) > maxSummary {
		text = string([]rune(text)[:maxSummary-1]) + "…"
	}

	replies := ""
	if t.Thread != nil && t.Thread.ReplyCount > 0 {
		replies = fmt.Sprintf("  %d repl%s", t.Thread.ReplyCount,
			map[bool]string{true: "y", false: "ies"}[t.Thread.ReplyCount == 1])
	}
	return text + m.styles.Timestamp.Render(replies)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
