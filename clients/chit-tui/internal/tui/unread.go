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

func (m *Model) computeUnread(channelID string, members []*model.ChannelMember) {
	if m.me == nil {
		return
	}
	var ch *model.Channel
	for _, c := range m.channels {
		if c.ID == channelID {
			ch = c
			break
		}
	}
	if ch == nil {
		for _, c := range m.dmChannels {
			if c.ID == channelID {
				ch = c
				break
			}
		}
	}
	if ch == nil {
		return
	}
	for _, mem := range members {
		if mem.UserID == m.me.ID {
			unread := ch.TotalMsgCount - mem.MsgCount
			if unread < 0 {
				unread = 0
			}
			m.setUnread(channelID, unread)
			return
		}
	}
}

func (m *Model) computeMentions(channelID string, members []*model.ChannelMember) {
	if m.me == nil {
		return
	}
	for _, mem := range members {
		if mem.UserID == m.me.ID {
			m.setMention(channelID, mem.MentionCount)
			return
		}
	}
}
