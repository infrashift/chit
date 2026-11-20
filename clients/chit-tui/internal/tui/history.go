package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
)

// threadInboxPageSize bounds the inbox at one screenful's worth. Following
// more threads than this is possible; paging through them is not yet.
const threadInboxPageSize = 50

// errNotSent reports a message typed before there was anywhere to send it.
func errNotSent() error { return errors.New("not sent: no channel is open yet") }

// errSearchHitNotLoaded reports a result that is outside the loaded history.
func errSearchHitNotLoaded() error {
	return errors.New(
		"that message is older than the loaded history; scroll back to reach it")
}

// closeThread returns from the thread pane to the channel view.
// clearSearchHighlight removes match highlighting. A highlight that outlives
// the search reads as if the term were still active.
func (m *Model) clearSearchHighlight() {
	assert.That(m != nil, "clearSearchHighlight: nil model")
	m.searchTerm = ""
	m.viewport.SetSearchTerm("")
}

func (m *Model) closeThread() tea.Cmd {
	assert.That(m.appState == AppStateRunning, "closeThread: not signed in")
	assert.That(m.mainPane == paneThread, "closeThread: no thread open")
	m.mainPane = paneChannel
	m.threadRootID = ""
	m.thread.Clear()
	cmd := m.setFocus(FocusViewport)
	m.resizeComponents()
	assert.That(m.focus == FocusViewport, "closeThread: history pane not focused")
	assert.That(m.mainPane == paneChannel, "closeThread: thread still shown")
	return cmd
}

// leaveThread closes the thread for a reason other than the reader asking,
// such as its root being deleted. Someone typing a reply keeps the input:
// moving them to the history pane would read the rest of their sentence as
// single-key commands.
func (m *Model) leaveThread() tea.Cmd {
	assert.That(m.appState == AppStateRunning, "leaveThread: not signed in")
	assert.That(m.mainPane == paneThread, "leaveThread: no thread open")
	if m.focus != FocusInput {
		return m.closeThread()
	}
	m.mainPane = paneChannel
	m.threadRootID = ""
	m.thread.Clear()
	m.resizeComponents()
	assert.That(m.focus == FocusInput, "leaveThread: input lost focus")
	assert.That(m.threadRootID == "", "leaveThread: thread root kept")
	return nil
}

// cancelEdit abandons a post edit in progress, along with its text. An edit
// left pending used to be applied by the next message sent, whatever
// channel it was sent in.
func (m *Model) cancelEdit() {
	assert.That(m != nil, "cancelEdit: nil model")
	if m.editingPostID == "" {
		return
	}
	m.editingPostID = ""
	m.input.SetValue("")
	assert.That(m.input.Value() == "", "cancelEdit: input kept the edit")
}

// errSearchHitElsewhere explains why the view changed channel.
func errSearchHitElsewhere() error {
	return errors.New("opened the channel containing that message")
}

// errLoadingOlder is a notice, not a failure: fetching a page of older
// history can take a moment and the view does not otherwise change.
func errLoadingOlder() error { return errors.New("loading older messages…") }

// postTypeCommandResponse marks the synthetic post the server returns when a
// message turns out to be a slash command. It is delivered separately as an
// ephemeral event and must not be appended twice.
const postTypeCommandResponse = "command_response"

// historyPageSize is how many posts a page of history holds. A short page
// means the server has no more to give.
const historyPageSize = 60

// maybeLoadOlder fetches the next page when the reader reaches the top of the
// loaded history. Without it the channel is capped at the first page and
// older messages are simply unreachable.
func (m *Model) maybeLoadOlder() tea.Cmd {
	assert.That(m != nil, "maybeLoadOlder: nil model")
	// An empty history pane is "at the top" while the channel loads, but
	// there is no first page yet to page back from.
	if m.activeChan == nil || m.loadingOlder || m.historyExhausted || m.viewport.Loading() {
		return nil
	}
	if !m.viewport.AtTop() {
		return nil
	}
	m.loadingOlder = true
	assert.That(m.appState == AppStateRunning, "maybeLoadOlder: not signed in")
	assert.That(m.scope != nil, "maybeLoadOlder: nil request scope")
	assert.That(m.historyPage >= 0, "maybeLoadOlder: negative history page")
	assert.That(m.client != nil, "maybeLoadOlder: nil client")
	return tea.Batch(
		FetchOlderPosts(m.reqCtx(), m.client, m.activeChan.ID, m.historyPage+1, historyPageSize),
		m.setError(errLoadingOlder()),
	)
}

// ownSelectedPost returns the selected post when the current user wrote it.
// The server refuses edits and deletes from anyone else, so the keys are
// inert on other people's messages rather than producing an error.
func (m Model) ownSelectedPost() *model.Post {
	assert.That(m.focus == FocusViewport, "ownSelectedPost: history pane not focused")
	assert.That(m.appState == AppStateRunning, "ownSelectedPost: not signed in")
	p := m.viewport.SelectedPost()
	if p == nil || m.me == nil || p.UserID != m.me.ID {
		return nil
	}
	return p
}

// postIDs collects the IDs of a page of posts.
func postIDs(posts []*model.Post) []string {
	ids := make([]string, 0, len(posts))
	for _, p := range posts {
		ids = append(ids, p.ID)
	}
	assert.That(len(ids) == len(posts), "postIDs: an ID for each post")
	return ids
}

// afterPostsLoaded starts what every newly shown page of history needs: its
// authors and its tags.
func (m *Model) afterPostsLoaded(posts []*model.Post) tea.Cmd {
	assert.That(m.appState == AppStateRunning, "afterPostsLoaded: not signed in")
	assert.That(m.client != nil, "afterPostsLoaded: nil client")
	assert.That(m.scope != nil, "afterPostsLoaded: nil request scope")
	assert.That(m.users != nil, "afterPostsLoaded: nil users cache")
	var cmds []tea.Cmd
	m.notePosts(posts)
	if ids := postIDs(posts); len(ids) > 0 {
		cmds = append(cmds, FetchPostsTags(m.reqCtx(), m.client, ids))
	}
	cmds = append(cmds, m.fetchMissingRefs())
	return tea.Batch(cmds...)
}

// handleSend posts what was typed: a reply in an open thread, an edit in
// progress, or a new message with its #tags.
func (m Model) handleSend(msg input.SendMsg) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "handleSend: nil request scope")
	assert.That(m.appState == AppStateRunning, "handleSend: not signed in")
	assert.That(m.client != nil, "handleSend: nil client")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "handleSend: unknown main pane")
	var cmds []tea.Cmd
	if m.activeChan == nil || m.me == nil {
		// The input has already cleared; put the text back so it is
		// not lost, and say why it went nowhere.
		m.input.SetValue(msg.Content)
		return m, m.setError(errNotSent())
	}
	// In the thread pane the input composes a reply to the thread root.
	// The root's ID is known from the moment the thread opens, so a
	// reply typed while it loads is still a reply.
	if m.mainPane == paneThread && m.threadRootID != "" {
		channelID := m.activeChan.ID
		if root := m.thread.RootPost(); root != nil && root.ChannelID != "" {
			channelID = root.ChannelID
		}
		reply := &model.Post{
			ChannelID: channelID,
			UserID:    m.me.ID,
			RootID:    m.threadRootID,
			Content:   msg.Content,
		}
		cmds = append(cmds, CreatePost(m.reqCtx(), m.client, reply))
		return m, tea.Batch(cmds...)
	}
	content, tagNames := tagpicker.StripHashtags(msg.Content)
	if content == "" && len(tagNames) > 0 {
		content = msg.Content
	}
	if m.editingPostID != "" {
		id := m.editingPostID
		m.editingPostID = ""
		cmds = append(cmds, EditPost(m.reqCtx(), m.client, id, content))
		cmds = append(cmds, m.tagPost(id, tagNames)...)
		return m, tea.Batch(cmds...)
	}

	post := &model.Post{
		ChannelID: m.activeChan.ID,
		UserID:    m.me.ID,
		Content:   content,
	}
	// The tags travel with the request, since only its response knows
	// the new post's ID.
	cmds = append(cmds, CreatePost(m.reqCtx(), m.client, post, tagNames...))
	return m, tea.Batch(cmds...)
}

// handlePostsLoaded shows a channel's first page of history, if it is for
// the channel still open.
func (m Model) handlePostsLoaded(msg PostsLoadedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "handlePostsLoaded: nil request scope")
	assert.That(m.appState == AppStateRunning, "handlePostsLoaded: not signed in")
	assert.That(m.client != nil, "handlePostsLoaded: nil client")
	if m.activeChan == nil || msg.ChannelID != m.activeChan.ID {
		// The reader moved on while this was in flight; applying it
		// would show one channel's history under another's name.
		return m, nil
	}
	m.viewport.SetLoading(false)
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	var page []*model.Post
	if msg.Posts != nil {
		page = msg.Posts.Order
	}
	m.viewport.SetPosts(page)
	m.historyPage = 0
	m.loadingOlder = false
	// A short first page means there is nothing older to ask for.
	m.historyExhausted = len(page) < historyPageSize
	m.postTags = make(map[string][]*model.Tag)
	load := m.afterPostsLoaded(page)
	if id := m.pendingJumpID; id != "" {
		m.pendingJumpID = ""
		m.viewport.SetSearchTerm(m.searchTerm)
		if !m.viewport.ScrollToPost(id) {
			return m, tea.Batch(load, m.setError(errSearchHitNotLoaded()))
		}
	}
	assert.That(m.historyPage == 0, "handlePostsLoaded: first page is not page 0")
	assert.That(!m.loadingOlder, "handlePostsLoaded: still loading an older page")
	return m, load
}

// handleOlderPosts puts an older page above the history, if it is the page
// that was asked for.
func (m Model) handleOlderPosts(msg OlderPostsLoadedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.client != nil, "handleOlderPosts: nil client")
	assert.That(m.appState == AppStateRunning, "handleOlderPosts: not signed in")
	assert.That(m.historyPage >= 0, "handleOlderPosts: negative history page")
	if m.activeChan == nil || msg.ChannelID != m.activeChan.ID || msg.Page != m.historyPage+1 {
		// The reader moved on, or the history was reloaded, while this
		// was in flight.
		return m, nil
	}
	assert.That(msg.Page >= 1, "handleOlderPosts: older page before page 1")
	m.loadingOlder = false
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	var older []*model.Post
	if msg.Posts != nil {
		older = msg.Posts.Order
	}
	if len(older) < historyPageSize {
		m.historyExhausted = true
	}
	if len(older) == 0 {
		return m, nil
	}
	m.historyPage = msg.Page
	m.viewport.PrependPosts(older)
	return m, m.afterPostsLoaded(older)
}

// handlePostCreated shows a sent post at once, rather than waiting for the
// WebSocket echo, and applies the tags it was sent with.
func (m Model) handlePostCreated(msg PostCreatedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "handlePostCreated: nil request scope")
	assert.That(m.appState == AppStateRunning, "handlePostCreated: not signed in")
	assert.That(m.client != nil, "handlePostCreated: nil client")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "handlePostCreated: unknown main pane")
	var cmds []tea.Cmd
	if msg.Err != nil {
		// The input cleared on Enter. Put the text back, unless something
		// new has been typed since, so a refused message can be fixed and
		// sent again rather than written out from scratch.
		if msg.Draft != "" && m.input.Value() == "" {
			m.input.SetValue(msg.Draft)
		}
		return m, m.setError(msg.Err)
	}
	// Show the message immediately rather than waiting for the WebSocket
	// echo. With the socket down the echo never arrives, so the input
	// cleared and the message simply vanished.
	if msg.Post != nil && msg.Post.Type != postTypeCommandResponse &&
		m.activeChan != nil && msg.Post.ChannelID == m.activeChan.ID &&
		!m.viewport.HasPost(msg.Post.ID) {
		m.viewport.AppendPost(msg.Post)
		m.notePosts([]*model.Post{msg.Post})
		cmds = append(cmds, m.fetchMissingRefs())
	}

	// Likewise a reply in the open thread: the echo is not guaranteed.
	if msg.Post != nil && msg.Post.RootID != "" && m.mainPane == paneThread &&
		msg.Post.RootID == m.threadRootID {
		m.thread.AppendReply(msg.Post)
	}
	if msg.Post != nil {
		cmds = append(cmds, m.tagPost(msg.Post.ID, msg.Tags)...)
	}
	return m, tea.Batch(cmds...)
}

// handleSearchHit jumps to a chosen search result, opening its channel
// first if need be.
func (m Model) handleSearchHit(msg palette.PostChosenMsg) (tea.Model, tea.Cmd) {
	assert.That(m.client != nil, "handleSearchHit: nil client")
	assert.That(m.appState == AppStateRunning, "handleSearchHit: not signed in")
	// Jumping to the hit is the point of searching; previously this only
	// moved focus and left the reader wherever they already were.
	cmd := m.setFocus(FocusViewport)
	assert.That(m.focus == FocusViewport, "handleSearchHit: history pane not focused")
	if msg.Post == nil {
		return m, cmd
	}
	// A result from another channel needs that channel opened first;
	// the post is not in the loaded history until it is.
	if ch := m.hitChannelToOpen(msg.Post); ch != nil {
		open := m.selectChannel(ch)
		assert.That(m.activeChan == ch, "handleSearchHit: hit's channel not opened")
		// The hit is selected once the channel's history arrives.
		m.pendingJumpID = msg.Post.ID
		return m, tea.Batch(cmd, open, m.setError(errSearchHitElsewhere()))
	}
	m.viewport.SetSearchTerm(m.searchTerm)
	if !m.viewport.ScrollToPost(msg.Post.ID) {
		// The hit is older than the posts held in memory. Say so
		// rather than silently doing nothing.
		return m, tea.Batch(cmd, m.setError(errSearchHitNotLoaded()))
	}
	return m, cmd
}

// hitChannelToOpen is the known channel a search hit is in, when that is
// not the open one; nil otherwise.
func (m Model) hitChannelToOpen(p *model.Post) *model.Channel {
	assert.That(p != nil, "hitChannelToOpen: nil post")
	if p.ChannelID == "" || (m.activeChan != nil && p.ChannelID == m.activeChan.ID) {
		return nil
	}
	ch := m.channelByID(p.ChannelID)
	assert.That(ch == nil || ch.ID == p.ChannelID, "hitChannelToOpen: found the wrong channel")
	return ch
}

// handleSearchSubmit runs a message search, in this channel or everywhere.
func (m Model) handleSearchSubmit(msg palette.SearchSubmitMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleSearchSubmit: not signed in")
	assert.That(m.client != nil, "handleSearchSubmit: nil client")
	assert.That(m.scope != nil, "handleSearchSubmit: nil request scope")
	var cmds []tea.Cmd
	term, tagNames := tagpicker.StripHashtags(msg.Term)
	var tagIDs []string
	for _, name := range tagNames {
		if t := m.searchTagByName(name, m.activeChannelID()); t != nil {
			tagIDs = append(tagIDs, t.ID)
		}
	}
	m.searchTerm = term
	m.searchWasGlobal = msg.Everywhere

	switch {
	case msg.Everywhere:
		cmds = append(cmds, SearchPostsEverywhere(m.reqCtx(), m.client, term, tagIDs))
	case m.activeChan != nil:
		cmds = append(cmds, SearchPosts(m.reqCtx(), m.client, m.activeChan.ID, term, tagIDs))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleSearchResults(msg SearchResultsMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleSearchResults: not signed in")
	assert.That(m.users != nil, "handleSearchResults: nil users cache")
	if msg.Term != m.searchTerm {
		return m, nil // superseded by a later search
	}
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	if msg.Posts != nil {
		m.palette.SetUsernames(m.usernameMap())
		m.palette.SetSearchResults(msg.Posts.Order)
	}
	return m, nil
}

func (m Model) handlePostPinned(msg PostPinnedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handlePostPinned: not signed in")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "handlePostPinned: unknown main pane")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	// Shown now rather than on the WebSocket echo, which never comes
	// with the socket down; the echo then changes nothing.
	m.viewport.SetPinned(msg.PostID, msg.Pinned)
	return m, nil
}

func (m Model) handlePostEdited(msg PostEditedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handlePostEdited: not signed in")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "handlePostEdited: unknown main pane")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	if msg.Post != nil {
		m.viewport.UpdatePost(msg.Post)
		m.thread.UpdatePost(msg.Post)
	}
	return m, nil
}

func (m Model) handlePostDeleted(msg PostDeletedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handlePostDeleted: not signed in")
	assert.That(m.mainPane == paneChannel || m.mainPane == paneThread, "handlePostDeleted: unknown main pane")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	m.viewport.RemovePost(msg.PostID)
	m.thread.RemoveReply(msg.PostID)
	return m, nil
}
