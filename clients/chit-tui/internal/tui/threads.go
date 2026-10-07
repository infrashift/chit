package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/threadinbox"
)

// handleThreadLoaded shows a thread, if it is still the one open.
func (m Model) handleThreadLoaded(msg ThreadLoadedMsg) (tea.Model, tea.Cmd) {
	if m.mainPane != paneThread || msg.PostID != m.threadRootID {
		// Another thread was opened, or this one closed, meanwhile.
		return m, nil
	}
	if msg.Err != nil {
		// An empty thread pane would take whatever is typed next and
		// post it to the channel, so go back to the channel instead.
		return m, tea.Batch(m.leaveThread(), m.setError(msg.Err))
	}
	if msg.Posts != nil && len(msg.Posts.Order) > 0 {
		root := msg.Posts.Order[0]
		var replies []*model.Post
		if len(msg.Posts.Order) > 1 {
			replies = msg.Posts.Order[1:]
		}
		m.thread.SetThread(root, replies)
		m.thread.SetUsernames(m.usernameMap())
		m.threadCounts[msg.PostID] = len(replies)
		m.viewport.SetThreadCounts(m.threadCounts)
		m.resizeComponents()
	}
	return m, nil
}

// handleThreadChosen opens a thread from the inbox, switching to its
// channel first.
func (m Model) handleThreadChosen(msg threadinbox.ThreadChosenMsg) (tea.Model, tea.Cmd) {
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
	if m.activeTeam != nil {
		cmds = append(cmds, MarkThreadRead(m.reqCtx(), m.client, m.activeTeam.ID, msg.RootID))
	}
	return m, tea.Batch(cmds...)
}
