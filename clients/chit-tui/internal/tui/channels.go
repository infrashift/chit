package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// minGroupChannelMembers matches the server's lower bound. Below it the
// conversation is a DM, which has its own flow.
const minGroupChannelMembers = 3

var errGroupTooSmall = errors.New(
	"a group needs at least three people — pick two others, or start a DM with @")

// selectChannel makes ch the active channel: marks the previous channel as
// viewed, loads posts, resets thread state, and derives the active team.
func (m *Model) selectChannel(ch *model.Channel) tea.Cmd {
	var cmds []tea.Cmd
	if m.activeChan != nil && m.activeChan.ID != ch.ID {
		cmds = append(cmds, ViewChannel(m.reqCtx(), m.client, m.activeChan.ID))
	}
	m.activeChan = ch
	m.channelAutoSelected = true
	m.pendingJumpID = ""
	m.cancelEdit()
	// A highlight from a search in the previous channel would otherwise
	// carry over and mark unrelated text here.
	m.clearSearchHighlight()
	// DM/group channels have no team; keep the last active team then.
	if ch.TeamID != "" {
		if t := m.teamByID(ch.TeamID); t != nil {
			m.activeTeam = t
		}
	}
	m.viewport.SetPosts(nil)
	m.viewport.SetLoading(true)
	// Paging state belongs to the channel being left.
	m.historyPage, m.loadingOlder, m.historyExhausted = 0, false, false
	cmds = append(cmds, FetchPosts(m.reqCtx(), m.client, ch.ID, 0, historyPageSize))
	cmds = append(cmds, ViewChannel(m.reqCtx(), m.client, ch.ID))
	// Members are only read for the active channel, to build the @-mention
	// list, so they are fetched on entry rather than for every channel in
	// every team up front.
	if _, have := m.channelMembers[ch.ID]; !have {
		cmds = append(cmds, FetchChannelMembers(m.reqCtx(), m.client, ch.ID))
	}
	m.mainPane = paneChannel
	m.thread.Clear()
	m.threadCounts = make(map[string]int)
	m.viewport.SetThreadCounts(m.threadCounts)
	m.resizeComponents()
	return tea.Batch(cmds...)
}

// addDMChannel prepends a DM/group channel to the list if it is not already
// present and refreshes derived display names.
func (m *Model) addDMChannel(ch *model.Channel) {
	for _, existing := range m.dmChannels {
		if existing.ID == ch.ID {
			m.resolveDMDisplayNames()
			return
		}
	}
	m.dmChannels = append([]*model.Channel{ch}, m.dmChannels...)
	m.palette.SetDMChannels(m.dmChannels)
	m.resolveDMDisplayNames()
}

// flattenChannels merges the per-team channel lists into one slice, in team
// order. Buckets are only ever keyed by known team IDs.
func (m Model) flattenChannels() []*model.Channel {
	var flat []*model.Channel
	for _, t := range m.teams {
		flat = append(flat, m.channelsByTeam[t.ID]...)
	}
	return flat
}

// teamByID returns the team with the given ID, or nil.
func (m Model) teamByID(id string) *model.Team {
	for _, t := range m.teams {
		if t.ID == id {
			return t
		}
	}
	return nil
}

func (m *Model) resolveDMDisplayNames() {
	if m.me == nil {
		return
	}
	for _, ch := range m.dmChannels {
		if name := m.peopleName(ch); name != "" {
			m.dmDisplayNames[ch.ID] = name
		}
		// Anyone not yet known is looked up, after which this runs again.
		for _, id := range m.peerIDs(ch) {
			if _, known := m.users[id]; !known {
				m.users[id] = nil
			}
		}
	}
}

// peerIDs returns the other members of a DM or group, read from its name,
// which joins the member IDs with "__".
func (m Model) peerIDs(ch *model.Channel) []string {
	if m.me == nil || (ch.Type != model.ChannelDirect && ch.Type != model.ChannelGroup) {
		return nil
	}
	parts := strings.Split(ch.Name, "__")
	if ch.Type == model.ChannelDirect && len(parts) != 2 {
		return nil
	}
	peers := make([]string, 0, len(parts))
	for _, id := range parts {
		if id != m.me.ID {
			peers = append(peers, id)
		}
	}
	return peers
}

// peopleName names a DM for the other person (display name, else username)
// and a group for its other members' usernames. It is "" until at least one
// of them has loaded.
func (m Model) peopleName(ch *model.Channel) string {
	var names []string
	for _, id := range m.peerIDs(ch) {
		u := m.users[id]
		if u == nil {
			continue
		}
		if ch.Type == model.ChannelDirect && u.DisplayName != "" {
			names = append(names, u.DisplayName)
		} else {
			names = append(names, u.Username)
		}
	}
	return strings.Join(names, ", ")
}

// activeChannelDisplayName resolves the human-readable name of the active
// channel: the other participant for DMs, the member list for group channels,
// and the display name otherwise.
func (m Model) activeChannelDisplayName() string {
	if m.activeChan == nil {
		return ""
	}
	// DMs and groups are named for the people in them, the same way the
	// palette names them.
	if name := m.peopleName(m.activeChan); name != "" {
		return name
	}
	return m.activeChan.DisplayName
}

// replaceChannel swaps an updated channel into every list holding it, so a
// rename shows up without a reload.
func (m *Model) replaceChannel(ch *model.Channel) {
	replace := func(list []*model.Channel) {
		for i, existing := range list {
			if existing.ID == ch.ID {
				list[i] = ch
			}
		}
	}
	replace(m.channels)
	replace(m.dmChannels)
	for _, list := range m.channelsByTeam {
		replace(list)
	}
	if m.activeChan != nil && m.activeChan.ID == ch.ID {
		m.activeChan = ch
	}
	m.palette.SetChannels(m.channels)
	m.palette.SetDMChannels(m.dmChannels)
}

// removeChannel drops a channel that no longer exists or is no longer
// reachable, moving off it first if it is the one being viewed.
func (m *Model) removeChannel(channelID string) []tea.Cmd {
	if channelID == "" {
		return nil
	}

	drop := func(list []*model.Channel) []*model.Channel {
		out := list[:0]
		for _, ch := range list {
			if ch.ID != channelID {
				out = append(out, ch)
			}
		}
		return out
	}
	m.channels = drop(m.channels)
	m.dmChannels = drop(m.dmChannels)
	for team, list := range m.channelsByTeam {
		m.channelsByTeam[team] = drop(list)
	}
	delete(m.channelMembers, channelID)
	delete(m.unread, channelID)
	delete(m.mentions, channelID)

	m.palette.SetChannels(m.channels)
	m.palette.SetDMChannels(m.dmChannels)

	var cmds []tea.Cmd
	if m.activeChan != nil && m.activeChan.ID == channelID {
		m.activeChan = nil
		m.viewport.SetPosts(nil)
		if len(m.channels) > 0 {
			cmds = append(cmds, m.selectChannel(m.channels[0]))
		}
		cmds = append(cmds, m.setError(errChannelGone))
	}
	return cmds
}

// errChannelGone explains why the view moved on its own.
var errChannelGone = errors.New("this channel is no longer available")

// channelByID finds a channel across the team lists and DMs. A search that
// spans every channel can return a hit from any of them.
func (m Model) channelByID(id string) *model.Channel {
	for _, ch := range m.channels {
		if ch.ID == id {
			return ch
		}
	}
	for _, ch := range m.dmChannels {
		if ch.ID == id {
			return ch
		}
	}
	for _, list := range m.channelsByTeam {
		for _, ch := range list {
			if ch.ID == id {
				return ch
			}
		}
	}
	return nil
}

// channelDisplayNames maps channel IDs to the names shown in the UI, so the
// thread inbox can say where each thread lives.
func (m Model) channelDisplayNames() map[string]string {
	names := make(map[string]string, len(m.channels)+len(m.dmChannels))
	for _, ch := range m.channels {
		names[ch.ID] = ch.DisplayName
	}
	for _, ch := range m.dmChannels {
		if name, ok := m.dmDisplayNames[ch.ID]; ok && name != "" {
			names[ch.ID] = name
			continue
		}
		names[ch.ID] = ch.DisplayName
	}
	return names
}
