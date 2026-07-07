package sidebar

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit-tui/internal/model"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
)

// ChannelSelectedMsg is sent when a channel is selected.
type ChannelSelectedMsg struct {
	Channel *model.Channel
}

// TeamSelectedMsg is sent when a team is selected.
type TeamSelectedMsg struct {
	Team *model.Team
}

// BackToTeamsMsg is sent when the user navigates back to the team list.
type BackToTeamsMsg struct{}

// Model is the sidebar component.
type Model struct {
	Teams          []*model.Team
	Channels       []*model.Channel
	DMChannels     []*model.Channel
	UnreadCounts   map[string]int64
	MentionCounts  map[string]int64
	dmDisplayNames map[string]string
	ActiveTeamID   string
	ActiveChanID   string
	teamCursor     int
	chanCursor     int
	showChannels   bool
	focused        bool
	width, height  int
	styles         styles.Styles
}

// New creates a new sidebar model.
func New(s styles.Styles) Model {
	return Model{
		UnreadCounts:   make(map[string]int64),
		MentionCounts:  make(map[string]int64),
		dmDisplayNames: make(map[string]string),
		styles:         s,
	}
}

// SetTeams sets the team list.
func (m *Model) SetTeams(teams []*model.Team) {
	m.Teams = teams
	if len(teams) > 0 && m.ActiveTeamID == "" {
		m.ActiveTeamID = teams[0].ID
		m.teamCursor = 0
	}
}

// SetChannels sets the channel list for the current team.
func (m *Model) SetChannels(channels []*model.Channel) {
	m.Channels = channels
	m.showChannels = true
	m.chanCursor = 0
	if len(channels) > 0 && m.ActiveChanID == "" {
		m.ActiveChanID = channels[0].ID
	}
}

// SetDMChannels sets the DM channel list.
func (m *Model) SetDMChannels(channels []*model.Channel) {
	m.DMChannels = channels
}

// SetDMDisplayName sets the display name for a DM channel.
func (m *Model) SetDMDisplayName(channelID, name string) {
	m.dmDisplayNames[channelID] = name
}

// GetDMDisplayName returns the display name for a DM/group channel.
func (m Model) GetDMDisplayName(channelID string) string {
	return m.dmDisplayNames[channelID]
}

// SetUnread sets the unread count for a channel.
func (m *Model) SetUnread(channelID string, count int64) {
	m.UnreadCounts[channelID] = count
}

// SetMention sets the mention count for a channel.
func (m *Model) SetMention(channelID string, count int64) {
	m.MentionCounts[channelID] = count
}

// Focus sets the focused state.
func (m *Model) Focus() { m.focused = true }

// Blur removes focus.
func (m *Model) Blur() { m.focused = false }

// Focused returns whether the sidebar is focused.
func (m Model) Focused() bool { return m.focused }

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets the width and height.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// totalItems returns the total number of navigable items (channels + DMs).
func (m Model) totalItems() int {
	return len(m.Channels) + len(m.DMChannels)
}

// SelectedChannel returns the currently selected channel (either regular or DM).
func (m Model) SelectedChannel() *model.Channel {
	if !m.showChannels {
		return nil
	}
	if m.chanCursor < len(m.Channels) {
		return m.Channels[m.chanCursor]
	}
	dmIdx := m.chanCursor - len(m.Channels)
	if dmIdx >= 0 && dmIdx < len(m.DMChannels) {
		return m.DMChannels[dmIdx]
	}
	return nil
}

// SelectedTeam returns the currently selected team.
func (m Model) SelectedTeam() *model.Team {
	if m.teamCursor < len(m.Teams) {
		return m.Teams[m.teamCursor]
	}
	return nil
}

// Update handles input messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.showChannels {
			if m.chanCursor > 0 {
				m.chanCursor--
			}
		} else {
			if m.teamCursor > 0 {
				m.teamCursor--
			}
		}
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.showChannels {
			if m.chanCursor < m.totalItems()-1 {
				m.chanCursor++
			}
		} else {
			if m.teamCursor < len(m.Teams)-1 {
				m.teamCursor++
			}
		}
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter"))):
		if m.showChannels {
			ch := m.SelectedChannel()
			if ch != nil {
				m.ActiveChanID = ch.ID
				return m, func() tea.Msg { return ChannelSelectedMsg{Channel: ch} }
			}
		} else {
			tm := m.SelectedTeam()
			if tm != nil {
				m.ActiveTeamID = tm.ID
				m.showChannels = true
				return m, func() tea.Msg { return TeamSelectedMsg{Team: tm} }
			}
		}
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("esc", "backspace"))):
		if m.showChannels {
			m.showChannels = false
			m.chanCursor = 0
			return m, func() tea.Msg { return BackToTeamsMsg{} }
		}
	}

	return m, nil
}

// View renders the sidebar.
func (m Model) View() string {
	borderStyle := m.styles.Sidebar
	if m.focused {
		borderStyle = borderStyle.BorderForeground(m.styles.ActiveBorder.GetBorderBottomForeground())
	}

	var items []string

	if m.showChannels {
		header := m.styles.SidebarActive.Render("Channels")
		items = append(items, header)

		for i, ch := range m.Channels {
			name := ch.DisplayName
			if name == "" {
				name = ch.Name
			}

			name = m.appendBadges(name, ch.ID)

			if i == m.chanCursor {
				items = append(items, m.styles.SidebarActive.Render("> "+name))
			} else if ch.ID == m.ActiveChanID {
				items = append(items, m.styles.SidebarActive.Render("  "+name))
			} else {
				items = append(items, m.styles.SidebarItem.Render("  "+name))
			}
		}

		// DM section
		if len(m.DMChannels) > 0 {
			items = append(items, "")
			dmHeader := m.styles.SidebarActive.Render("Direct Messages")
			items = append(items, dmHeader)

			for i, ch := range m.DMChannels {
				name := m.dmDisplayNames[ch.ID]
				if name == "" {
					name = ch.DisplayName
					if name == "" {
						name = ch.Name
					}
				}

				name = m.appendBadges(name, ch.ID)

				cursorIdx := len(m.Channels) + i
				if cursorIdx == m.chanCursor {
					items = append(items, m.styles.SidebarActive.Render("> "+name))
				} else if ch.ID == m.ActiveChanID {
					items = append(items, m.styles.SidebarActive.Render("  "+name))
				} else {
					items = append(items, m.styles.SidebarItem.Render("  "+name))
				}
			}
		}
	} else {
		header := m.styles.SidebarActive.Render("Teams")
		items = append(items, header)

		for i, tm := range m.Teams {
			name := tm.DisplayName
			if name == "" {
				name = tm.Name
			}
			if i == m.teamCursor {
				items = append(items, m.styles.SidebarActive.Render("> "+name))
			} else {
				items = append(items, m.styles.SidebarItem.Render("  "+name))
			}
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return borderStyle.Width(m.width - 2).Height(m.height - 2).Render(content)
}

func (m Model) appendBadges(name, channelID string) string {
	unread := m.UnreadCounts[channelID]
	if unread > 0 {
		name += " " + m.styles.UnreadBadge.Render(fmt.Sprintf("(%d)", unread))
	}
	mentionCount := m.MentionCounts[channelID]
	if mentionCount > 0 {
		name += " " + m.styles.MentionBadge.Render(fmt.Sprintf("[@%d]", mentionCount))
	}
	return name
}
