package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/threadinbox"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

// handleThreadLoaded shows a thread, if it is still the one open.
func (m Model) handleThreadLoaded(msg ThreadLoadedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleThreadLoaded: not signed in")
	if m.mainPane != paneThread || msg.PostID != m.threadRootID {
		// Another thread was opened, or this one closed, meanwhile.
		return m, nil
	}
	if msg.Err != nil {
		// An empty thread pane would take whatever is typed next and
		// post it to the channel, so go back to the channel instead.
		return m, tea.Batch(m.leaveThread(), m.setError(msg.Err))
	}
	var cmd tea.Cmd
	if msg.Posts != nil && len(msg.Posts.Order) > 0 {
		root := msg.Posts.Order[0]
		var replies []*model.Post
		if len(msg.Posts.Order) > 1 {
			replies = msg.Posts.Order[1:]
		}
		m.thread.SetThread(root, replies)
		m.pushPeople()
		m.pushFiles()
		m.notePosts(msg.Posts.Order)
		cmd = m.fetchMissingRefs()
		assert.That(len(replies) == len(msg.Posts.Order)-1, "handleThreadLoaded: replies lost")
		assert.That(m.threadCounts != nil, "handleThreadLoaded: nil thread counts")
		m.threadCounts[msg.PostID] = len(replies)
		m.viewport.SetThreadCounts(m.threadCounts)
		m.resizeComponents()
	}
	return m, cmd
}

// handleThreadChosen opens a thread from the inbox, switching to its
// channel first.
func (m Model) handleThreadChosen(msg threadinbox.ThreadChosenMsg) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "handleThreadChosen: nil request scope")
	assert.That(m.appState == AppStateRunning, "handleThreadChosen: not signed in")
	assert.That(m.client != nil, "handleThreadChosen: nil client")
	var cmds []tea.Cmd
	// Opening a thread means switching to its channel first: the thread
	// pane renders against the active channel, and the thread may well be
	// in one the user is not currently looking at.
	if ch := m.channelByID(msg.ChannelID); ch != nil && (m.activeChan == nil || m.activeChan.ID != ch.ID) {
		cmds = append(cmds, m.selectChannel(ch))
	}
	m.cancelEdit()
	cmds = append(cmds, FetchThread(m.reqCtx(), m.client, msg.RootID))
	m.mainPane = paneThread
	m.threadRootID = msg.RootID
	cmds = append(cmds, m.setFocus(FocusThread))
	m.resizeComponents()
	cmds = append(cmds, MarkThreadRead(m.reqCtx(), m.client, msg.RootID))
	assert.That(m.mainPane == paneThread, "handleThreadChosen: thread not shown")
	assert.That(m.focus == FocusThread, "handleThreadChosen: thread not focused")
	assert.That(m.editingPostID == "", "handleThreadChosen: edit kept")
	return m, tea.Batch(cmds...)
}

// handlePostSelected opens the thread of the post picked in the history.
func (m Model) handlePostSelected(msg viewport.PostSelectedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "handlePostSelected: nil request scope")
	assert.That(msg.Post != nil, "handlePostSelected: nil post")
	assert.That(m.appState == AppStateRunning, "handlePostSelected: not signed in")
	assert.That(m.client != nil, "handlePostSelected: nil client")
	var cmds []tea.Cmd
	m.cancelEdit()
	// Replies are listed in the channel too. Opening one opens its
	// root's thread: a thread rooted at a reply is not one the server
	// recognizes, and it refuses replies to a reply.
	root := msg.Post.ID
	if msg.Post.RootID != "" {
		root = msg.Post.RootID
	}
	cmds = append(cmds, FetchThread(m.reqCtx(), m.client, root))
	m.mainPane = paneThread
	m.threadRootID = root
	cmds = append(cmds, m.setFocus(FocusInput))
	m.resizeComponents()
	assert.That(m.mainPane == paneThread, "handlePostSelected: thread not shown")
	assert.That(m.focus == FocusInput, "handlePostSelected: input not focused")
	assert.That(m.editingPostID == "", "handlePostSelected: edit kept")
	return m, tea.Batch(cmds...)
}

func (m Model) handleThreadsLoaded(msg ThreadsLoadedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleThreadsLoaded: not signed in")
	if msg.Err != nil {
		m.threadInbox.SetThreads(nil, nil)
		return m, m.setError(msg.Err)
	}
	m.threadInbox.SetThreads(msg.Threads, msg.Direct)
	return m, nil
}
