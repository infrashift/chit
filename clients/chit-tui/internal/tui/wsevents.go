package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
)

func (m Model) handleWSEvent(msg WebSocketEventMsg) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "handleWSEvent: nil request scope")
	assert.That(m.appState == AppStateRunning, "handleWSEvent: not signed in")
	cmds := []tea.Cmd{ListenWebSocket(m.wsClient)}
	cmds = append(cmds, m.applyWSEvent(msg.Event)...)
	return m, tea.Batch(cmds...)
}

// applyWSEvent applies one server event to the model and returns the
// requests it calls for.
func (m *Model) applyWSEvent(evt model.WebSocketEvent) []tea.Cmd {
	assert.That(m != nil, "applyWSEvent: nil model")
	assert.That(m.appState == AppStateRunning, "applyWSEvent: not signed in")
	switch evt.Event {
	case model.WebSocketEventPosted:
		return m.onPosted(evt)
	case model.WebSocketEventPostEdited:
		m.onPostEdited(evt)
	case model.WebSocketEventPostPinned, model.WebSocketEventPostUnpinned:
		// Unlike an edit, these carry only an ID, so the flag is flipped on
		// the post already held rather than replacing it.
		if postID := dataString(evt.Data, "post_id"); postID != "" {
			m.viewport.SetPinned(postID, evt.Event == model.WebSocketEventPostPinned)
		}
	case model.WebSocketEventPostDeleted:
		return m.onPostDeleted(evt)
	case model.WebSocketEventChannelUpdated:
		if ch := decodeChannel(evt.Data); ch != nil {
			m.replaceChannel(ch)
		}
	case model.WebSocketEventChannelDeleted:
		if channelID := dataString(evt.Data, "channel_id"); channelID != "" {
			return m.removeChannel(channelID)
		}
	case model.WebSocketEventUserRemoved:
		return m.onUserRemoved(evt)
	case model.WebSocketEventCommandResponse:
		m.onCommandResponse(evt)
	case model.WebSocketEventPostTagsUpdated:
		m.onPostTagsUpdated(evt)
	case model.WebSocketEventThreadUpdated:
		m.onThreadUpdated(evt)
	case model.WebSocketEventMentioned:
		m.onMentioned(evt)
	case model.WebSocketEventUserAdded:
		return m.onUserAdded(evt)
	case model.WebSocketEventChannelCreated:
		return m.onChannelCreated(evt)
	case model.WebSocketEventStreamStatus:
		return m.onStreamStatus(evt)
	}
	return nil
}

// onPosted shows a new post in the open channel, or counts it unread in
// any other.
func (m *Model) onPosted(evt model.WebSocketEvent) []tea.Cmd {
	assert.That(m.appState == AppStateRunning, "onPosted: not signed in")
	assert.That(m.unread != nil, "onPosted: nil unread counts")
	p := decodePost(evt.Data)
	if p == nil {
		// A payload the client cannot read means the two sides disagree
		// about the schema. Dropped silently, that looks like messages
		// simply never arriving.
		slog.Warn("could not decode a posted event", "event", evt.Event)
		return nil
	}
	if m.activeChan != nil && p.ChannelID == m.activeChan.ID {
		return m.appendLivePost(p)
	}
	if m.me == nil || p.UserID != m.me.ID {
		// Your own post, sent from another device, is not unread.
		m.setUnread(p.ChannelID, m.unread[p.ChannelID]+1)
	}
	return nil
}

// appendLivePost adds a post that arrived in the open channel.
func (m *Model) appendLivePost(p *model.Post) []tea.Cmd {
	assert.That(p != nil, "appendLivePost: nil post")
	assert.That(p.ID != "", "appendLivePost: post without ID")
	assert.That(m.activeChan != nil, "appendLivePost: no channel open")
	assert.That(p.ChannelID == m.activeChan.ID, "appendLivePost: post from another channel")
	// The reply of a streamed answer has arrived.
	m.endStreamsOf(p.UserID)
	// Reply counts are left to thread_updated, which carries the
	// server's total; counting here as well over-counted each reply.
	if p.RootID != "" && m.mainPane == paneThread && p.RootID == m.threadRootID {
		m.thread.AppendReply(p) // ignores a reply already shown
	}
	// The sender already appended this from the HTTP response.
	if m.viewport.HasPost(p.ID) {
		return nil
	}
	m.viewport.AppendPost(p)
	m.notePosts([]*model.Post{p})
	assert.That(m.viewport.HasPost(p.ID), "appendLivePost: post not shown")
	return []tea.Cmd{m.fetchMissingRefs()}
}

func (m *Model) onPostEdited(evt model.WebSocketEvent) {
	assert.That(m.appState == AppStateRunning, "onPostEdited: not signed in")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "onPostEdited: unknown main pane")
	// An edit carries the whole post, so it replaces what is displayed.
	p := decodePost(evt.Data)
	if p == nil {
		slog.Warn("could not decode a post_edited event")
		return
	}
	assert.That(p.ID != "", "onPostEdited: decoded post without ID")
	m.viewport.UpdatePost(p)
	if m.mainPane == paneThread {
		m.thread.UpdatePost(p)
	}
}

func (m *Model) onPostDeleted(evt model.WebSocketEvent) []tea.Cmd {
	assert.That(m.appState == AppStateRunning, "onPostDeleted: not signed in")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "onPostDeleted: unknown main pane")
	postID := dataString(evt.Data, "post_id")
	if postID == "" {
		return nil
	}
	m.viewport.RemovePost(postID)
	// Leave a thread whose root just disappeared, rather than
	// showing an empty pane.
	if m.mainPane == paneThread && m.thread.RootPost() != nil &&
		m.thread.RootPost().ID == postID {
		return []tea.Cmd{m.leaveThread()}
	}
	if m.mainPane == paneThread {
		m.thread.RemoveReply(postID)
	}
	return nil
}

func (m *Model) onUserRemoved(evt model.WebSocketEvent) []tea.Cmd {
	assert.That(m.scope != nil, "onUserRemoved: nil request scope")
	assert.That(m.appState == AppStateRunning, "onUserRemoved: not signed in")
	assert.That(m.client != nil, "onUserRemoved: nil client")
	channelID := dataString(evt.Data, "channel_id")
	userID := dataString(evt.Data, "user_id")
	if m.me != nil && userID == m.me.ID {
		// Removed from a channel: it is no longer reachable.
		return m.removeChannel(channelID)
	}
	if channelID == "" {
		return nil
	}
	// Someone else left; the @-mention list is now stale.
	return m.dropChannelMembers(channelID)
}

// dropChannelMembers forgets a channel's member list after someone joined
// or left it, and reloads it if the channel is open.
func (m *Model) dropChannelMembers(channelID string) []tea.Cmd {
	assert.That(channelID != "", "dropChannelMembers: empty channel ID")
	assert.That(m.channelMembers != nil, "dropChannelMembers: nil members cache")
	delete(m.channelMembers, channelID)
	assert.That(m.channelMembers[channelID] == nil, "dropChannelMembers: members kept")
	if m.activeChan != nil && m.activeChan.ID == channelID {
		return []tea.Cmd{FetchChannelMembers(m.reqCtx(), m.client, channelID)}
	}
	return nil
}

func (m *Model) onCommandResponse(evt model.WebSocketEvent) {
	assert.That(m.appState == AppStateRunning, "onCommandResponse: not signed in")
	assert.That(m.commandAuthors != nil, "onCommandResponse: nil command authors")
	// Ephemeral: broadcast to the invoking user only, never persisted.
	// It is shown as a post so multi-line output such as /help stays
	// readable, and it disappears on the next channel load.
	text := dataString(evt.Data, "text")
	channelID := dataString(evt.Data, "channel_id")
	slug := dataString(evt.Data, "command_slug")

	if text == "" || m.activeChan == nil || channelID != m.activeChan.ID {
		return
	}
	author := m.commandAuthor(slug)
	// Each response needs its own ID: the render cache is keyed by
	// it, so a reused one showed the first output again.
	m.commandResponses++
	assert.That(m.commandResponses > 0, "onCommandResponse: response counter wrapped")
	assert.That(author != "", "onCommandResponse: no author")
	m.viewport.AppendPost(&model.Post{
		ID:        fmt.Sprintf("%s:%d", author, m.commandResponses),
		ChannelID: channelID,
		UserID:    author,
		Content:   text,
		CreateAt:  time.Now().UnixMilli(),
	})
	m.pushPeople()
}

func (m *Model) onPostTagsUpdated(evt model.WebSocketEvent) {
	assert.That(m.appState == AppStateRunning, "onPostTagsUpdated: not signed in")
	assert.That(m.postTags != nil, "onPostTagsUpdated: nil post tags")
	// Tags are applied after a post is created, so a post arrives
	// untagged and its tags follow in this event. It carries the whole
	// list, which replaces what is shown.
	postID := dataString(evt.Data, "post_id")
	channelID := dataString(evt.Data, "channel_id")
	if postID == "" || m.activeChan == nil || channelID != m.activeChan.ID {
		return
	}
	var tags []*model.Tag
	if raw, ok := evt.Data["tags"]; ok && raw != nil {
		if err := reDecode(raw, &tags); err != nil {
			slog.Warn("could not decode a post_tags_updated payload", "error", err)
			return
		}
	}
	m.postTags[postID] = tags
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	assert.That(len(names) == len(tags), "onPostTagsUpdated: a name for each tag")
	m.viewport.SetPostsTags(map[string][]string{postID: names})
	m.thread.SetPostTags(m.postTags)
}

func (m *Model) onThreadUpdated(evt model.WebSocketEvent) {
	assert.That(m.appState == AppStateRunning, "onThreadUpdated: not signed in")
	assert.That(m.threadCounts != nil, "onThreadUpdated: nil thread counts")
	threadData, ok := evt.Data["thread"]
	if !ok {
		return
	}
	var t model.Thread
	if err := reDecode(threadData, &t); err != nil {
		slog.Warn("could not decode a thread_updated payload", "error", err)
		return
	}
	m.threadCounts[t.PostID] = t.ReplyCount
	m.viewport.SetThreadCounts(m.threadCounts)
}

func (m *Model) onMentioned(evt model.WebSocketEvent) {
	assert.That(m.appState == AppStateRunning, "onMentioned: not signed in")
	assert.That(m.mentions != nil, "onMentioned: nil mention counts")
	// The server names the channel in the data; the broadcast only
	// addresses the mentioned user. Reading the channel from the
	// broadcast found nothing, so live mentions were never counted.
	chanID := dataString(evt.Data, "channel_id")
	if chanID == "" && evt.Broadcast != nil {
		chanID = evt.Broadcast.ChannelID
	}
	// A mention in the channel being read is seen as it arrives.
	if chanID != "" && (m.activeChan == nil || chanID != m.activeChan.ID) {
		m.setMention(chanID, m.mentions[chanID]+1)
	}
}

func (m *Model) onUserAdded(evt model.WebSocketEvent) []tea.Cmd {
	assert.That(m.scope != nil, "onUserAdded: nil request scope")
	assert.That(m.appState == AppStateRunning, "onUserAdded: not signed in")
	assert.That(m.client != nil, "onUserAdded: nil client")
	userID := dataString(evt.Data, "user_id")
	channelID := dataString(evt.Data, "channel_id")
	switch {
	case m.me != nil && userID == m.me.ID:
		// The event names the channel but not its team, so every team
		// is re-read, and the DMs in case it is a group.
		var cmds []tea.Cmd
		for _, t := range m.teams {
			cmds = append(cmds, FetchChannels(m.reqCtx(), m.client, t.ID))
		}
		return append(cmds, FetchDMChannels(m.reqCtx(), m.client))
	case channelID != "":
		// Someone else joined: the @-mention list for it is stale.
		return m.dropChannelMembers(channelID)
	}
	return nil
}

func (m *Model) onChannelCreated(evt model.WebSocketEvent) []tea.Cmd {
	assert.That(m.client != nil, "onChannelCreated: nil client")
	assert.That(m.scope != nil, "onChannelCreated: nil request scope")
	kind := dataString(evt.Data, "type")
	teamID := dataString(evt.Data, "team_id")
	switch kind {
	case model.ChannelDirect, model.ChannelGroup:
		return []tea.Cmd{FetchDMChannels(m.reqCtx(), m.client)}
	case model.ChannelOpen, model.ChannelPrivate:
		// Any of the user's teams, not only the one in view.
		if m.teamByID(teamID) != nil {
			return []tea.Cmd{FetchChannels(m.reqCtx(), m.client, teamID)}
		}
	}
	return nil
}

// reDecode round-trips a decoded event field back through JSON into a typed
// value. Event payloads arrive as generic maps, so this is how they are read
// without asserting field by field.
func reDecode(v any, out any) error {
	assert.That(out != nil, "reDecode: nil destination")
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
	assert.That(m.client != nil, "handleWSState: nil client")
	assert.That(m.appState == AppStateRunning, "handleWSState: not signed in")
	assert.That(m.scope != nil, "handleWSState: nil request scope")
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
	assert.That(m.appState == AppStateRunning, "handleWSConnected: not signed in")
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
	assert.That(m.wsListening, "handleWSConnected: listeners not started")
	if msg.Err != nil {
		// The client keeps redialing, and WSStateMsg reports when it
		// gets through.
		cmds = append(cmds, m.setError(fmt.Errorf("real-time updates are offline, retrying: %w", msg.Err)))
	}
	return m, tea.Batch(cmds...)
}

// dataString returns the string at key in an event's data, or "" when the
// key is missing or holds something else.
func dataString(data map[string]any, key string) string {
	assert.That(key != "", "dataString: empty key")
	s, ok := data[key].(string)
	if !ok {
		return ""
	}
	return s
}
