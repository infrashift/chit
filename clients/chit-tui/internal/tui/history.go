package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// threadInboxPageSize bounds the inbox at one screenful's worth. Following
// more threads than this is possible; paging through them is not yet.
const threadInboxPageSize = 50

// errNotSent reports a message typed before there was anywhere to send it.
var errNotSent = errors.New("not sent: no channel is open yet")

// errSearchHitNotLoaded reports a result that is outside the loaded history.
var errSearchHitNotLoaded = errors.New(
	"that message is older than the loaded history; scroll back to reach it")

// closeThread returns from the thread pane to the channel view.
// clearSearchHighlight removes match highlighting. A highlight that outlives
// the search reads as if the term were still active.
func (m *Model) clearSearchHighlight() {
	m.searchTerm = ""
	m.viewport.SetSearchTerm("")
}

func (m *Model) closeThread() tea.Cmd {
	m.mainPane = paneChannel
	m.threadRootID = ""
	m.thread.Clear()
	cmd := m.setFocus(FocusViewport)
	m.resizeComponents()
	return cmd
}

// leaveThread closes the thread for a reason other than the reader asking,
// such as its root being deleted. Someone typing a reply keeps the input:
// moving them to the history pane would read the rest of their sentence as
// single-key commands.
func (m *Model) leaveThread() tea.Cmd {
	if m.focus != FocusInput {
		return m.closeThread()
	}
	m.mainPane = paneChannel
	m.threadRootID = ""
	m.thread.Clear()
	m.resizeComponents()
	return nil
}

// cancelEdit abandons a post edit in progress, along with its text. An edit
// left pending used to be applied by the next message sent, whatever
// channel it was sent in.
func (m *Model) cancelEdit() {
	if m.editingPostID == "" {
		return
	}
	m.editingPostID = ""
	m.input.SetValue("")
}

// errSearchHitElsewhere explains why the view changed channel.
var errSearchHitElsewhere = errors.New("opened the channel containing that message")

// errLoadingOlder is a notice, not a failure: fetching a page of older
// history can take a moment and the view does not otherwise change.
var errLoadingOlder = errors.New("loading older messages…")

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
	// An empty history pane is "at the top" while the channel loads, but
	// there is no first page yet to page back from.
	if m.activeChan == nil || m.loadingOlder || m.historyExhausted || m.viewport.Loading() {
		return nil
	}
	if !m.viewport.AtTop() {
		return nil
	}
	m.loadingOlder = true
	return tea.Batch(
		FetchOlderPosts(m.reqCtx(), m.client, m.activeChan.ID, m.historyPage+1, historyPageSize),
		m.setError(errLoadingOlder),
	)
}

// ownSelectedPost returns the selected post when the current user wrote it.
// The server refuses edits and deletes from anyone else, so the keys are
// inert on other people's messages rather than producing an error.
func (m Model) ownSelectedPost() *model.Post {
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
	return ids
}

// afterPostsLoaded starts what every newly shown page of history needs: its
// authors and its tags.
func (m *Model) afterPostsLoaded(posts []*model.Post) tea.Cmd {
	var cmds []tea.Cmd
	m.resolvePostUsers(posts)
	if ids := postIDs(posts); len(ids) > 0 {
		cmds = append(cmds, FetchPostsTags(m.reqCtx(), m.client, ids))
	}
	if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
		cmds = append(cmds, fetchCmd)
	}
	return tea.Batch(cmds...)
}
