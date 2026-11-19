package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/chcreator"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/login"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/mention"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/threadinbox"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
)

// Update processes messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "Update: nil request scope")
	// Always handle window size regardless of state.
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsm.Width
		m.height = wsm.Height
		m.loginModel.SetSize(wsm.Width, wsm.Height)
		m.resizeComponents()
		return m, nil
	}

	// Handle login/re-login state.
	if m.appState == AppStateLogin || m.appState == AppStateReLogin {
		return m.updateLogin(msg)
	}
	assert.That(m.appState == AppStateRunning, "Update: not signed in")
	assert.That(m.client != nil, "Update: nil client")

	// A rejected session is the same problem whichever request noticed it, so
	// it is caught once here. Previously only five message types checked,
	// and every other request showed a transient toast on expiry and left
	// the user in a client that could no longer talk to the server.
	if err := msgError(msg); err != nil && api.IsUnauthorized(err) {
		return m.handleAuthExpired()
	}

	if next, cmd, ok := m.routeSessionMsg(msg); ok {
		return next, cmd
	}
	if next, cmd, ok := m.routeChannelMsg(msg); ok {
		return next, cmd
	}
	if next, cmd, ok := m.routePostMsg(msg); ok {
		return next, cmd
	}
	if next, cmd, ok := m.routeTagMsg(msg); ok {
		return next, cmd
	}
	if next, cmd, ok := m.routeLookupMsg(msg); ok {
		return next, cmd
	}
	return m, nil
}

// handled marks a message as consumed by one of the routers below, which
// report whether the message was theirs so Update can try the next.
func handled(next tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd, bool) {
	assert.That(next != nil, "handled: nil model")
	return next, cmd, true
}

// ackRequest handles a request whose success needs no further work: only a
// failure is worth showing.
func (m Model) ackRequest(err error) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "ackRequest: not signed in")
	if err != nil {
		return m, m.setError(err)
	}
	return m, nil
}

// routeSessionMsg routes input, connection, account and status messages.
func (m Model) routeSessionMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "routeSessionMsg: not signed in")
	assert.That(m.scope != nil, "routeSessionMsg: nil request scope")
	switch msg := msg.(type) {
	case login.LoginSuccessMsg:
		return handled(m.handleLoginSuccess(msg))
	case login.LoginErrorMsg:
		var cmd tea.Cmd
		m.loginModel, cmd = m.loginModel.Update(msg)
		return m, cmd, true
	case tea.MouseMsg:
		return handled(m.handleMouse(msg))
	case tea.KeyMsg:
		return handled(m.handleKey(msg))
	case UserLoadedMsg:
		return handled(m.handleUserLoaded(msg))
	case WSStateMsg:
		return handled(m.handleWSState(msg))
	case WSConnectedMsg:
		return handled(m.handleWSConnected(msg))
	case WebSocketEventMsg:
		return handled(m.handleWSEvent(msg))
	case ProfileUpdatedMsg:
		return handled(m.handleProfileUpdated(msg))
	case skinpicker.SkinSelectedMsg:
		return handled(m.handleThemeChosen(msg))
	case clipboardCopiedMsg:
		return handled(m.handleClipboardCopied(msg))
	case ClearErrMsg:
		assert.That(msg.Seq <= m.errSeq, "routeSessionMsg: clear for an error not yet set")
		if msg.Seq == m.errSeq {
			m.err = nil
		}
		return m, nil, true
	case ErrMsg:
		return handled(m.handleErr(msg))
	}
	return m, nil, false
}

// routeChannelMsg routes messages about teams, channels and their members.
func (m Model) routeChannelMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "routeChannelMsg: not signed in")
	assert.That(m.client != nil, "routeChannelMsg: nil client")
	assert.That(m.scope != nil, "routeChannelMsg: nil request scope")
	switch msg := msg.(type) {
	case TeamsLoadedMsg:
		return handled(m.handleTeamsLoaded(msg))
	case ChannelsLoadedMsg:
		return handled(m.handleChannelsLoaded(msg))
	case ChannelMembersLoadedMsg:
		return handled(m.handleChannelMembersLoaded(msg))
	case MyChannelMembersLoadedMsg:
		return handled(m.handleMyChannelMembersLoaded(msg))
	case ChannelViewedMsg:
		return handled(m.handleChannelViewed(msg))
	case palette.ChannelChosenMsg:
		return handled(m.handleChannelChosen(msg))
	case palette.UserChosenMsg:
		return handled(m.handleUserChosen(msg))
	case DMChannelsLoadedMsg:
		return handled(m.handleDMChannelsLoaded(msg))
	case DMCreatedMsg:
		return handled(m.handleDMCreated(msg))
	case chcreator.ChannelSubmittedMsg:
		return handled(m.handleChannelSubmitted(msg))
	case dmpicker.CancelledMsg:
		return handled(m.handleMemberPickCancelled(msg))
	case dmpicker.MembersPickedMsg:
		return handled(m.handleMembersPicked(msg))
	case ChannelCreatedMsg:
		return handled(m.handleChannelCreated(msg))
	case AllMembersAddedMsg:
		return m, tea.Batch(FetchChannelMembers(m.reqCtx(), m.client, msg.ChannelID)), true
	case ChannelMemberAddedMsg:
		return handled(m.ackRequest(msg.Err))
	case ChannelLeftMsg:
		return handled(m.handleChannelLeft(msg))
	}
	return m, nil, false
}

// routePostMsg routes messages about posts, threads and sending.
func (m Model) routePostMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "routePostMsg: not signed in")
	assert.That(m.client != nil, "routePostMsg: nil client")
	assert.That(m.scope != nil, "routePostMsg: nil request scope")
	switch msg := msg.(type) {
	case PostsLoadedMsg:
		return handled(m.handlePostsLoaded(msg))
	case OlderPostsLoadedMsg:
		return handled(m.handleOlderPosts(msg))
	case PostCreatedMsg:
		return handled(m.handlePostCreated(msg))
	case PostPinnedMsg:
		return handled(m.handlePostPinned(msg))
	case PostEditedMsg:
		return handled(m.handlePostEdited(msg))
	case PostDeletedMsg:
		return handled(m.handlePostDeleted(msg))
	case viewport.PostSelectedMsg:
		return handled(m.handlePostSelected(msg))
	case input.SendMsg:
		return handled(m.handleSend(msg))
	case input.SlashTriggerMsg:
		return handled(m.handleSlash(msg))
	case ThreadLoadedMsg:
		return handled(m.handleThreadLoaded(msg))
	case ThreadsLoadedMsg:
		return handled(m.handleThreadsLoaded(msg))
	case ThreadFollowChangedMsg:
		return handled(m.ackRequest(msg.Err))
	case threadinbox.ThreadChosenMsg:
		return handled(m.handleThreadChosen(msg))
	case threadinbox.FollowToggledMsg:
		return m, SetThreadFollowing(m.reqCtx(), m.client, msg.RootID, msg.Following), true
	case threadinbox.ClosedMsg:
		return m, m.setFocus(FocusInput), true
	}
	return m, nil, false
}

// routeTagMsg routes messages about tags and the tag picker.
func (m Model) routeTagMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "routeTagMsg: not signed in")
	assert.That(m.client != nil, "routeTagMsg: nil client")
	assert.That(m.scope != nil, "routeTagMsg: nil request scope")
	switch msg := msg.(type) {
	case AllTagsLoadedMsg:
		return handled(m.handleAllTagsLoaded(msg))
	case PostTagsLoadedMsg:
		return handled(m.handlePostTagsLoaded(msg))
	case PostsTagsLoadedMsg:
		return handled(m.handlePostsTagsLoaded(msg))
	case tagpicker.TagToggledMsg:
		return handled(m.handleTagToggled(msg))
	case tagpicker.TagCreateRequestMsg:
		return m, tea.Batch(CreateTagAndApplyCmd(m.reqCtx(), m.client, msg.Name, msg.PostID, m.activeChannelID(), msg.Private)), true
	case TagCreatedMsg:
		return handled(m.handleTagCreated(msg))
	case TagAddedToPostMsg:
		return handled(m.handleTagAddedToPost(msg))
	case TagRemovedFromPostMsg:
		return handled(m.handleTagRemovedFromPost(msg))
	}
	return m, nil, false
}

// routeLookupMsg routes the palette, search, mention and user-search
// messages: everything that looks something up for the user.
func (m Model) routeLookupMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	assert.That(m.appState == AppStateRunning, "routeLookupMsg: not signed in")
	assert.That(m.client != nil, "routeLookupMsg: nil client")
	assert.That(m.scope != nil, "routeLookupMsg: nil request scope")
	switch msg := msg.(type) {
	case CommandsLoadedMsg:
		return handled(m.handleCommandsLoaded(msg))
	case palette.CommandChosenMsg:
		return handled(m.handleCommandChosen(msg))
	case palette.UserQueryMsg:
		return handled(m.searchUsers(msg.Term))
	case dmpicker.SearchTriggeredMsg:
		return handled(m.searchUsers(msg.Term))
	case palette.DebounceMsg:
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd, true
	case palette.SearchSubmitMsg:
		return handled(m.handleSearchSubmit(msg))
	case SearchResultsMsg:
		return handled(m.handleSearchResults(msg))
	case palette.PostChosenMsg:
		return handled(m.handleSearchHit(msg))
	case input.AtTriggerMsg:
		return handled(m.handleAtTrigger(msg))
	case input.AtDismissMsg:
		m.mention.Hide()
		return m, nil, true
	case mention.UserSelectedMsg:
		m.input.ReplaceAtMention(msg.StartCol, msg.Username)
		m.mention.Hide()
		return m, nil, true
	case UsersLoadedMsg:
		return handled(m.handleUsersLoaded(msg))
	case UserSearchResultsMsg:
		return handled(m.handleUserSearchResults(msg))
	}
	return m, nil, false
}
