package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// Connection-state notices. They travel through the same transient status
// line as errors because that is the only place the UI can say something
// in passing.
var (
	errDisconnected = errors.New("connection lost — reconnecting")
	errReconnected  = errors.New("reconnected — reloading messages")
	errDesynced     = errors.New("fell behind the server — reloading messages")
	// Not a failure, but the status line is the only place to ask.
	errConfirmDelete = errors.New("delete this message? Press d again to confirm, any other key to cancel")
)

func (m Model) handleWSEvent(msg WebSocketEventMsg) (tea.Model, tea.Cmd) {
	evt := msg.Event

	var cmds []tea.Cmd
	cmds = append(cmds, ListenWebSocket(m.wsClient))

	switch evt.Event {
	case model.WebSocketEventPosted:
		p := decodePost(evt.Data)
		if p == nil {
			// A payload the client cannot read means the two sides disagree
			// about the schema. Dropped silently, that looks like messages
			// simply never arriving.
			slog.Warn("could not decode a posted event", "event", evt.Event)
			return m, tea.Batch(cmds...)
		}
		if m.activeChan != nil && p.ChannelID == m.activeChan.ID {
			// Reply counts are left to thread_updated, which carries the
			// server's total; counting here as well over-counted each reply.
			if p.RootID != "" && m.mainPane == paneThread && p.RootID == m.threadRootID {
				m.thread.AppendReply(p) // ignores a reply already shown
			}
			// The sender already appended this from the HTTP response.
			if m.viewport.HasPost(p.ID) {
				return m, tea.Batch(cmds...)
			}
			m.viewport.AppendPost(p)
			m.resolvePostUsers([]*model.Post{p})
			if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
				cmds = append(cmds, fetchCmd)
			}
		} else if m.me == nil || p.UserID != m.me.ID {
			// Your own post, sent from another device, is not unread.
			m.setUnread(p.ChannelID, m.unread[p.ChannelID]+1)
		}

	case model.WebSocketEventPostEdited:
		// An edit carries the whole post, so it replaces what is displayed.
		if p := decodePost(evt.Data); p != nil {
			m.viewport.UpdatePost(p)
			if m.mainPane == paneThread {
				m.thread.UpdatePost(p)
			}
		} else {
			slog.Warn("could not decode a post_edited event")
		}

	case model.WebSocketEventPostPinned, model.WebSocketEventPostUnpinned:
		// Unlike an edit, these carry only an ID, so the flag is flipped on
		// the post already held rather than replacing it.
		if postID, _ := evt.Data["post_id"].(string); postID != "" {
			m.viewport.SetPinned(postID, evt.Event == model.WebSocketEventPostPinned)
		}

	case model.WebSocketEventPostDeleted:
		postID, _ := evt.Data["post_id"].(string)
		if postID != "" {
			m.viewport.RemovePost(postID)
			// Leave a thread whose root just disappeared, rather than
			// showing an empty pane.
			if m.mainPane == paneThread && m.thread.RootPost() != nil &&
				m.thread.RootPost().ID == postID {
				cmds = append(cmds, m.leaveThread())
			} else if m.mainPane == paneThread {
				m.thread.RemoveReply(postID)
			}
		}

	case model.WebSocketEventChannelUpdated:
		if ch := decodeChannel(evt.Data); ch != nil {
			m.replaceChannel(ch)
		}

	case model.WebSocketEventChannelDeleted:
		channelID, _ := evt.Data["channel_id"].(string)
		if channelID != "" {
			cmds = append(cmds, m.removeChannel(channelID)...)
		}

	case model.WebSocketEventUserRemoved:
		channelID, _ := evt.Data["channel_id"].(string)
		userID, _ := evt.Data["user_id"].(string)
		if m.me != nil && userID == m.me.ID {
			// Removed from a channel: it is no longer reachable.
			cmds = append(cmds, m.removeChannel(channelID)...)
		} else if channelID != "" {
			// Someone else left; the @-mention list is now stale.
			delete(m.channelMembers, channelID)
			if m.activeChan != nil && m.activeChan.ID == channelID {
				cmds = append(cmds, FetchChannelMembers(m.reqCtx(), m.client, channelID))
			}
		}

	case model.WebSocketEventCommandResponse:
		// Ephemeral: broadcast to the invoking user only, never persisted.
		// It is shown as a post so multi-line output such as /help stays
		// readable, and it disappears on the next channel load.
		text, _ := evt.Data["text"].(string)
		channelID, _ := evt.Data["channel_id"].(string)
		slug, _ := evt.Data["command_slug"].(string)

		if text != "" && m.activeChan != nil && channelID == m.activeChan.ID {
			author := m.commandAuthor(slug)
			// Each response needs its own ID: the render cache is keyed by
			// it, so a reused one showed the first output again.
			m.commandResponses++
			m.viewport.AppendPost(&model.Post{
				ID:        fmt.Sprintf("%s:%d", author, m.commandResponses),
				ChannelID: channelID,
				UserID:    author,
				Content:   text,
				CreateAt:  time.Now().UnixMilli(),
			})
			m.viewport.SetUsernames(m.usernameMap())
		}

	case model.WebSocketEventPostTagsUpdated:
		// Tags are applied after a post is created, so a post arrives
		// untagged and its tags follow in this event. It carries the whole
		// list, which replaces what is shown.
		postID, _ := evt.Data["post_id"].(string)
		channelID, _ := evt.Data["channel_id"].(string)
		if postID == "" || m.activeChan == nil || channelID != m.activeChan.ID {
			break
		}
		var tags []*model.Tag
		if raw, ok := evt.Data["tags"]; ok && raw != nil {
			if err := reDecode(raw, &tags); err != nil {
				slog.Warn("could not decode a post_tags_updated payload", "error", err)
				break
			}
		}
		m.postTags[postID] = tags
		names := make([]string, 0, len(tags))
		for _, t := range tags {
			names = append(names, t.Name)
		}
		m.viewport.SetPostsTags(map[string][]string{postID: names})
		m.thread.SetPostTags(m.postTags)

	case model.WebSocketEventThreadUpdated:
		if threadData, ok := evt.Data["thread"]; ok {
			var t model.Thread
			if err := reDecode(threadData, &t); err != nil {
				slog.Warn("could not decode a thread_updated payload", "error", err)
			} else {
				m.threadCounts[t.PostID] = t.ReplyCount
				m.viewport.SetThreadCounts(m.threadCounts)
			}
		}

	case model.WebSocketEventMentioned:
		// The server names the channel in the data; the broadcast only
		// addresses the mentioned user. Reading the channel from the
		// broadcast found nothing, so live mentions were never counted.
		chanID, _ := evt.Data["channel_id"].(string)
		if chanID == "" && evt.Broadcast != nil {
			chanID = evt.Broadcast.ChannelID
		}
		// A mention in the channel being read is seen as it arrives.
		if chanID != "" && (m.activeChan == nil || chanID != m.activeChan.ID) {
			m.setMention(chanID, m.mentions[chanID]+1)
		}

	case model.WebSocketEventUserAdded:
		userID, _ := evt.Data["user_id"].(string)
		channelID, _ := evt.Data["channel_id"].(string)
		switch {
		case m.me != nil && userID == m.me.ID:
			// The event names the channel but not its team, so every team
			// is re-read, and the DMs in case it is a group.
			for _, t := range m.teams {
				cmds = append(cmds, FetchChannels(m.reqCtx(), m.client, t.ID))
			}
			cmds = append(cmds, FetchDMChannels(m.reqCtx(), m.client))
		case channelID != "":
			// Someone else joined: the @-mention list for it is stale.
			delete(m.channelMembers, channelID)
			if m.activeChan != nil && m.activeChan.ID == channelID {
				cmds = append(cmds, FetchChannelMembers(m.reqCtx(), m.client, channelID))
			}
		}

	case model.WebSocketEventChannelCreated:
		kind, _ := evt.Data["type"].(string)
		teamID, _ := evt.Data["team_id"].(string)
		switch kind {
		case model.ChannelDirect, model.ChannelGroup:
			cmds = append(cmds, FetchDMChannels(m.reqCtx(), m.client))
		case model.ChannelOpen, model.ChannelPrivate:
			// Any of the user's teams, not only the one in view.
			if m.teamByID(teamID) != nil {
				cmds = append(cmds, FetchChannels(m.reqCtx(), m.client, teamID))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// reDecode round-trips a decoded event field back through JSON into a typed
// value. Event payloads arrive as generic maps, so this is how they are read
// without asserting field by field.
func reDecode(v any, out any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// decodePost re-decodes an event payload into a post. Event data arrives as a
// generic map, so it is round-tripped through JSON rather than asserted field
// by field.
func decodePost(data map[string]any) *model.Post {
	var p model.Post
	if err := reDecode(data, &p); err != nil || p.ID == "" {
		return nil
	}
	return &p
}

// decodeChannel re-decodes an event payload into a channel.
func decodeChannel(data map[string]any) *model.Channel {
	var c model.Channel
	if err := reDecode(data, &c); err != nil || c.ID == "" {
		return nil
	}
	return &c
}

// handleWSState reacts to the socket connecting, dropping, or falling
// behind.
func (m Model) handleWSState(msg WSStateMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	// Keep listening for the next transition before doing anything else,
	// or a single drop would be the last one ever reported.
	cmds = append(cmds, ListenWSState(m.wsClient))

	if msg.Unauthorized {
		m.wsConnected = false
		updated, cmd := m.handleAuthExpired()
		return updated, tea.Batch(append(cmds, cmd)...)
	}

	wasConnected := m.wsConnected
	m.wsConnected = msg.Connected

	// Events were dropped while the socket stayed up, so the view is
	// stale with nothing else to reveal it. Same remedy as a reconnect:
	// re-read the channel. Handled before the transition checks, which
	// would otherwise see no change and do nothing.
	if msg.Desynced {
		if m.activeChan != nil {
			cmds = append(cmds, FetchPosts(m.reqCtx(), m.client, m.activeChan.ID, 0, historyPageSize))
		}
		cmds = append(cmds, m.setError(errDesynced))
		return m, tea.Batch(cmds...)
	}

	if msg.Connected && !wasConnected {
		// Events that arrived while the socket was down are gone for
		// good — the stream has no replay — so re-read the channel
		// rather than leaving a silent hole in the history.
		if m.activeChan != nil {
			cmds = append(cmds, FetchPosts(m.reqCtx(), m.client, m.activeChan.ID, 0, historyPageSize))
		}
		cmds = append(cmds, m.setError(errReconnected))
	}
	if !msg.Connected && wasConnected {
		cmds = append(cmds, m.setError(errDisconnected))
	}
	return m, tea.Batch(cmds...)
}

// handleWSConnected starts the listeners after the first dial, and reports
// a failed one.
func (m Model) handleWSConnected(msg WSConnectedMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if errors.Is(msg.Err, ws.ErrUnauthorized) {
		return m.handleAuthExpired()
	}
	m.wsConnected = msg.Err == nil
	// Both listeners start here: one for events, one for transport
	// state. Returning only the first is what left disconnects silent.
	// The client's channels outlive a sign-out, so the listeners from
	// the first session keep serving every later one; starting another
	// pair on each sign-in would leave several reading the same stream.
	if !m.wsListening {
		m.wsListening = true
		cmds = append(cmds, ListenWebSocket(m.wsClient), ListenWSState(m.wsClient))
	}
	if msg.Err != nil {
		// The client keeps redialing, and WSStateMsg reports when it
		// gets through.
		cmds = append(cmds, m.setError(fmt.Errorf("real-time updates are offline, retrying: %w", msg.Err)))
	}
	return m, tea.Batch(cmds...)
}
