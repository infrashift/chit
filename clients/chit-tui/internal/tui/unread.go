package tui

import "github.com/infrashift/chit/clients/chit-tui/internal/model"

// UnreadCount returns the unread count for a channel.
func (m Model) UnreadCount(channelID string) int64 { return m.unread[channelID] }

// MentionCount returns the mention count for a channel.
func (m Model) MentionCount(channelID string) int64 { return m.mentions[channelID] }

// setUnread updates the unread count for a channel.
func (m *Model) setUnread(channelID string, count int64) {
	m.unread[channelID] = count
}

// setMention updates the mention count for a channel.
func (m *Model) setMention(channelID string, count int64) {
	m.mentions[channelID] = count
}

// applyMembers sets a channel's badges from its member rows, using the
// signed-in user's row.
func (m *Model) applyMembers(channelID string, members []*model.ChannelMember) {
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
	if m.activeChan != nil && mem.ChannelID == m.activeChan.ID {
		m.setUnread(mem.ChannelID, 0)
		m.setMention(mem.ChannelID, 0)
		return
	}
	ch := m.channelByID(mem.ChannelID)
	if ch == nil {
		return
	}
	m.setUnread(mem.ChannelID, max(ch.TotalMsgCount-mem.MsgCount, 0))
	m.setMention(mem.ChannelID, mem.MentionCount)
}
