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
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/listwin"
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
	m := Model{styles: s, channelNames: map[string]string{}}
	assert.That(!m.visible && !m.loading, "threadinbox.New: a new inbox starts hidden and idle")
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.New: team count outside the list")
	return m
}

// Open shows the overlay in its loading state: the list is fetched when it
// opens, and an empty box would otherwise read as "no threads".
func (m *Model) Open() {
	m.visible = true
	m.focused = true
	m.loading = true
	m.cursor = 0
	m.threads = nil
	// The team section is empty until the list arrives.
	m.teamCount = 0
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.Open: team count outside the list")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.threads)), "threadinbox.Open: cursor outside the list")
	assert.That(!m.loading || len(m.threads) == 0, "threadinbox.Open: threads listed while loading")
	assert.That(m.channelNames != nil, "threadinbox.Open: nil channel name map")
}

// Close hides the overlay.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.loading = false
	assert.That(!m.visible && !m.focused, "threadinbox.Close: still shown")
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.Close: team count outside the list")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.threads)), "threadinbox.Close: cursor outside the list")
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
	assert.That(len(m.threads) == len(team)+len(direct), "threadinbox.SetThreads: threads lost joining the lists")
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.SetThreads: team count outside the list")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.threads)), "threadinbox.SetThreads: cursor outside the list")
	assert.That(!m.loading || len(m.threads) == 0, "threadinbox.SetThreads: threads listed while loading")
}

// SetTeamName names the team whose threads head the list.
func (m *Model) SetTeamName(name string) { m.teamName = name }

// isDirect reports whether row i is in the direct-message section.
func (m Model) isDirect(i int) bool {
	assert.That(i >= 0 && i < len(m.threads), "threadinbox.isDirect: row outside the list")
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.isDirect: team count outside the list")
	return i >= m.teamCount
}

// SetChannelNames supplies display names keyed by channel ID.
func (m *Model) SetChannelNames(names map[string]string) {
	assert.That(names != nil, "threadinbox.SetChannelNames: nil map")
	m.channelNames = names
}

// Threads returns the current list, for callers that need to look one up.
func (m Model) Threads() []*model.ThreadResponse {
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.Threads: team count outside the list")
	assert.That(!m.loading || len(m.threads) == 0, "threadinbox.Threads: threads listed while loading")
	return m.threads
}

// selected returns the thread under the cursor, or nil.
func (m Model) selected() *model.ThreadResponse {
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.threads)), "threadinbox.selected: cursor outside the list")
	assert.That(!m.loading || len(m.threads) == 0, "threadinbox.selected: threads listed while loading")
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
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.Update: team count outside the list")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.threads)), "threadinbox.Update: cursor outside the list")
	assert.That(!m.loading || len(m.threads) == 0, "threadinbox.Update: threads listed while loading")

	switch keyMsg.String() {
	case "esc":
		m.Close()
		assert.That(!m.visible, "threadinbox.Update: still shown after esc")
		return m, func() tea.Msg { return ClosedMsg{} }
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		assert.That(m.cursor >= 0, "threadinbox.Update: cursor above the first row")
		return m, nil
	case "down", "j":
		if m.cursor < len(m.threads)-1 {
			m.cursor++
		}
		assert.That(m.cursor == 0 || m.cursor < len(m.threads), "threadinbox.Update: cursor below the last row")
		return m, nil
	case "u":
		return m.unfollow()
	case "enter":
		t := m.selected()
		if t == nil || t.Thread == nil {
			return m, nil
		}
		root, channel := t.Thread.PostID, t.Thread.ChannelID
		m.Close()
		assert.That(!m.visible && !m.focused, "threadinbox.Update: still shown after opening a thread")
		return m, func() tea.Msg { return ThreadChosenMsg{RootID: root, ChannelID: channel} }
	}
	return m, nil
}

// unfollow drops the selected thread out of the list, as the server will,
// rather than leaving a row that no longer belongs, and reports the change.
func (m Model) unfollow() (Model, tea.Cmd) {
	t := m.selected()
	if t == nil || t.Thread == nil {
		return m, nil
	}
	assert.That(m.cursor < len(m.threads), "threadinbox.unfollow: selection outside the list")
	before, beforeTeam := len(m.threads), m.teamCount
	id := t.Thread.PostID
	if !m.isDirect(m.cursor) {
		m.teamCount--
	}
	m.threads = append(m.threads[:m.cursor], m.threads[m.cursor+1:]...)
	if m.cursor >= len(m.threads) {
		m.cursor = max(len(m.threads)-1, 0)
	}
	assert.That(len(m.threads) == before-1, "threadinbox.unfollow: not exactly one thread removed")
	assert.That(m.teamCount == beforeTeam || m.teamCount == beforeTeam-1, "threadinbox.unfollow: team count moved by more than one")
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.unfollow: team count outside the list")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.threads)), "threadinbox.unfollow: cursor outside the list")
	return m, func() tea.Msg { return FollowToggledMsg{RootID: id, Following: false} }
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
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.View: team count outside the list")
	assert.That(!m.loading || len(m.threads) == 0, "threadinbox.View: threads listed while loading")

	switch {
	case m.loading:
		items = append(items, "", m.styles.Timestamp.Render("  Loading…"))
	case len(m.threads) == 0:
		items = append(items, "",
			m.styles.Timestamp.Render("  You are not following any threads."),
			m.styles.Timestamp.Render("  Reply to a message to start following its thread."))
	default:
		// Two rows go to the section headings, so the window leaves room.
		start, end := listwin.Window(m.cursor, len(m.threads), max(listwin.Rows(m.height)-2, 3))
		assert.That(start <= m.cursor && m.cursor < end, "threadinbox.View: cursor outside the window")
		assert.That(end <= len(m.threads), "threadinbox.View: window ends past the list")
		// Scrolled into a section, its heading is out of view; keep the gap.
		if (start > 0 || m.teamCount == 0) && start != m.teamCount {
			items = append(items, "")
		}
		for i := start; i < end; i++ {
			// A heading starts each non-empty section.
			if i == 0 && m.teamCount > 0 {
				items = append(items, "", m.styles.Timestamp.Render("  "+m.teamHeading()))
			}
			if i == m.teamCount {
				items = append(items, "", m.styles.Timestamp.Render("  Direct messages"))
			}
			items = append(items, m.renderRow(i, m.threads[i]))
		}
		assert.That(len(items) > end-start, "threadinbox.View: a visible row went missing")
		items = append(items, "",
			m.styles.Timestamp.Render("  enter to open · u to unfollow · esc to close"))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(min(m.width*2/3, 78)).Render(body)
}

func (m Model) renderRow(i int, t *model.ThreadResponse) string {
	assert.That(i >= 0 && i < len(m.threads), "threadinbox.renderRow: row outside the list")
	assert.That(t == m.threads[i], "threadinbox.renderRow: thread is not row i's")
	assert.That(m.visible, "threadinbox.renderRow: rendered while hidden")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.threads)), "threadinbox.renderRow: cursor outside the list")
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
	assert.That(m.teamCount > 0, "threadinbox.teamHeading: heading for an empty team section")
	assert.That(m.visible, "threadinbox.teamHeading: rendered while hidden")
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.teamHeading: team count outside the list")
	if m.teamName == "" {
		return "In this team"
	}
	return "In " + m.teamName
}

// channelLabel says where row i's thread lives: #channel in the team section,
// and who the conversation is with in the direct section, where a # would
// suggest a channel that does not exist.
func (m Model) channelLabel(i int, t *model.ThreadResponse) string {
	assert.That(i >= 0 && i < len(m.threads) && t == m.threads[i], "threadinbox.channelLabel: thread is not row i's")
	assert.That(m.channelNames != nil, "threadinbox.channelLabel: nil channel name map")
	assert.That(m.teamCount >= 0 && m.teamCount <= len(m.threads), "threadinbox.channelLabel: team count outside the list")
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
	assert.That(m.visible, "threadinbox.summary: rendered while hidden")
	root := t.Root()
	if root == nil || strings.TrimSpace(root.Content) == "" {
		return m.styles.Timestamp.Render("(no message)")
	}

	text := strings.Join(strings.Fields(root.Content), " ")
	const maxSummary = 48
	if len([]rune(text)) > maxSummary {
		text = string([]rune(text)[:maxSummary-1]) + "…"
	}
	assert.That(text != "", "threadinbox.summary: non-blank root summarized as empty")
	assert.That(len([]rune(text)) <= maxSummary, "threadinbox.summary: summary not truncated")

	replies := ""
	if t.Thread != nil && t.Thread.ReplyCount > 0 {
		replies = fmt.Sprintf("  %d repl%s", t.Thread.ReplyCount,
			map[bool]string{true: "y", false: "ies"}[t.Thread.ReplyCount == 1])
	}
	return text + m.styles.Timestamp.Render(replies)
}

func plural(n int) string {
	assert.That(n > 0, "threadinbox.plural: count of nothing")
	if n == 1 {
		return ""
	}
	return "s"
}
