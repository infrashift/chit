package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// UnreadCount returns the unread count for a channel.
func (m Model) UnreadCount(channelID string) int64 {
	assert.That(m.unread != nil, "UnreadCount: nil unread counts")
	return m.unread[channelID]
}

// MentionCount returns the mention count for a channel.
func (m Model) MentionCount(channelID string) int64 {
	assert.That(m.mentions != nil, "MentionCount: nil mention counts")
	return m.mentions[channelID]
}

// setUnread updates the unread count for a channel.
func (m *Model) setUnread(channelID string, count int64) {
	assert.That(m.unread != nil, "setUnread: nil unread counts")
	assert.That(count >= 0, "setUnread: negative count")
	m.unread[channelID] = count
}

// setMention updates the mention count for a channel.
func (m *Model) setMention(channelID string, count int64) {
	assert.That(m != nil, "setMention: nil model")
	assert.That(m.mentions != nil, "setMention: nil mention counts")
	m.mentions[channelID] = count
}

// applyMembers sets a channel's badges from its member rows, using the
// signed-in user's row.
func (m *Model) applyMembers(channelID string, members []*model.ChannelMember) {
	assert.That(m.appState == AppStateRunning, "applyMembers: not signed in")
	assert.That(m.unread != nil, "applyMembers: nil unread counts")
	if m.me == nil {
		return
	}
	for _, mem := range members {
		if mem.UserID == m.me.ID {
			m.applyMyMembership(mem)
			return
		}
	}
}

// applyMyMembership sets a channel's unread and mention badges from the
// signed-in user's member row. The open channel is being read, so it stays
// at zero: a row fetched alongside marking it viewed can predate the view
// and would otherwise bring the old count back.
func (m *Model) applyMyMembership(mem *model.ChannelMember) {
	assert.That(m.unread != nil, "applyMyMembership: nil unread counts")
	if m.activeChan != nil && mem.ChannelID == m.activeChan.ID {
		m.setUnread(mem.ChannelID, 0)
		m.setMention(mem.ChannelID, 0)
		assert.That(m.unread[mem.ChannelID] == 0, "applyMyMembership: open channel counted unread")
		return
	}
	ch := m.channelByID(mem.ChannelID)
	if ch == nil {
		return
	}
	m.setUnread(mem.ChannelID, max(ch.TotalMsgCount-mem.MsgCount, 0))
	assert.That(m.unread[mem.ChannelID] >= 0, "applyMyMembership: negative unread count")
	m.setMention(mem.ChannelID, mem.MentionCount)
}

func (m Model) handleChannelMembersLoaded(msg ChannelMembersLoadedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleChannelMembersLoaded: not signed in")
	assert.That(m.channelMembers != nil, "handleChannelMembersLoaded: nil members cache")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	m.channelMembers[msg.ChannelID] = msg.Members
	m.applyMembers(msg.ChannelID, msg.Members)
	return m, nil
}

func (m Model) handleMyChannelMembersLoaded(msg MyChannelMembersLoadedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.mentions != nil, "handleMyChannelMembersLoaded: nil mention counts")
	assert.That(m.appState == AppStateRunning, "handleMyChannelMembersLoaded: not signed in")
	assert.That(m.unread != nil, "handleMyChannelMembersLoaded: nil unread counts")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	for _, mem := range msg.Members {
		m.applyMyMembership(mem)
	}
	return m, nil
}

func (m Model) handleChannelViewed(msg ChannelViewedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleChannelViewed: not signed in")
	if msg.Err != nil {
		// Marking read failed, so the badge is about to disagree with
		// what the reader just did. Saying so beats leaving them to
		// wonder why the channel still looks unread.
		return m, m.setError(fmt.Errorf("could not mark the channel read: %w", msg.Err))
	}
	m.setUnread(msg.ChannelID, 0)
	m.setMention(msg.ChannelID, 0)
	assert.That(m.unread[msg.ChannelID] == 0, "handleChannelViewed: unread badge kept")
	assert.That(m.mentions[msg.ChannelID] == 0, "handleChannelViewed: mention badge kept")
	return m, nil
}
