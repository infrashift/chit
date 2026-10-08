package tui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
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
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// Update processes messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

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

	// A rejected session is the same problem whichever request noticed it, so
	// it is caught once here. Previously only five message types checked,
	// and every other request showed a transient toast on expiry and left
	// the user in a client that could no longer talk to the server.
	if err := msgError(msg); err != nil && api.IsUnauthorized(err) {
		return m.handleAuthExpired()
	}

	switch msg := msg.(type) {
	case login.LoginSuccessMsg:
		return m.handleLoginSuccess(msg)

	case login.LoginErrorMsg:
		m.loginModel, _ = m.loginModel.Update(msg)
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case UserLoadedMsg:
		return m.handleUserLoaded(msg)

	case TeamsLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.teams = msg.Teams
		m.palette.SetTeams(msg.Teams)
		if len(msg.Teams) > 0 {
			m.activeTeam = msg.Teams[0]
		}
		for _, t := range msg.Teams {
			cmds = append(cmds, FetchChannels(m.reqCtx(), m.client, t.ID))
		}
		return m, tea.Batch(cmds...)

	case ChannelsLoadedMsg:
		return m.handleChannelsLoaded(msg)

	case PostsLoadedMsg:
		return m.handlePostsLoaded(msg)

	case PostCreatedMsg:
		return m.handlePostCreated(msg)

	case ThreadLoadedMsg:
		return m.handleThreadLoaded(msg)

	case CommandsLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.palette.SetCommands(mergeCommands(clientCommands(), msg.Commands))
		return m, nil

	case UsersLoadedMsg:
		return m.handleUsersLoaded(msg)

	case ChannelMembersLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.channelMembers[msg.ChannelID] = msg.Members
		m.applyMembers(msg.ChannelID, msg.Members)
		return m, nil

	case MyChannelMembersLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		for _, mem := range msg.Members {
			m.applyMyMembership(mem)
		}
		return m, nil

	case ChannelViewedMsg:
		if msg.Err != nil {
			// Marking read failed, so the badge is about to disagree with
			// what the reader just did. Saying so beats leaving them to
			// wonder why the channel still looks unread.
			return m, m.setError(fmt.Errorf("could not mark the channel read: %w", msg.Err))
		}
		m.setUnread(msg.ChannelID, 0)
		m.setMention(msg.ChannelID, 0)
		return m, nil

	case WSStateMsg:
		return m.handleWSState(msg)

	case WSConnectedMsg:
		return m.handleWSConnected(msg)

	case WebSocketEventMsg:
		return m.handleWSEvent(msg)

	case viewport.PostSelectedMsg:
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
		return m, tea.Batch(cmds...)

	case input.SendMsg:
		return m.handleSend(msg)

	case input.SlashTriggerMsg:
		return m.handleSlash(msg)

	case palette.ChannelChosenMsg:
		cmds = append(cmds, m.selectChannel(msg.Channel))
		cmds = append(cmds, m.setFocus(FocusInput))
		return m, tea.Batch(cmds...)

	case palette.CommandChosenMsg:
		// Insert rather than execute: most commands take arguments, and the
		// palette has no way to collect them. The user completes the line and
		// presses Enter, which sends it like any other message.
		if msg.Command != nil {
			m.input.SetValue("/" + msg.Command.Slug + " ")
		}
		return m, m.setFocus(FocusInput)

	case palette.UserChosenMsg:
		if m.me != nil {
			cmds = append(cmds, CreateDMChannel(m.reqCtx(), m.client, m.me.ID, msg.User.ID))
		}
		return m, tea.Batch(cmds...)

	case palette.UserQueryMsg:
		m.userQuery = msg.Term
		cmds = append(cmds, SearchUsersCmd(m.reqCtx(), m.client, msg.Term))
		return m, tea.Batch(cmds...)

	case palette.DebounceMsg:
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd

	case palette.SearchSubmitMsg:
		return m.handleSearchSubmit(msg)

	case SearchResultsMsg:
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

	case palette.PostChosenMsg:
		return m.handleSearchHit(msg)

	case input.AtTriggerMsg:
		entries := m.buildMentionEntries()
		m.mention.SetEntries(entries)
		m.mention.Show(msg.Prefix, msg.StartCol)
		return m, nil

	case input.AtDismissMsg:
		m.mention.Hide()
		return m, nil

	case mention.UserSelectedMsg:
		m.input.ReplaceAtMention(msg.StartCol, msg.Username)
		m.mention.Hide()
		return m, nil

	case DMChannelsLoadedMsg:
		return m.handleDMChannelsLoaded(msg)

	case dmpicker.SearchTriggeredMsg:
		m.userQuery = msg.Term
		cmds = append(cmds, SearchUsersCmd(m.reqCtx(), m.client, msg.Term))
		return m, tea.Batch(cmds...)

	case UserSearchResultsMsg:
		return m.handleUserSearchResults(msg)

	case ThreadsLoadedMsg:
		if msg.Err != nil {
			m.threadInbox.SetThreads(nil, nil)
			return m, m.setError(msg.Err)
		}
		m.threadInbox.SetThreads(msg.Threads, msg.Direct)
		return m, nil

	case ThreadFollowChangedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		return m, nil

	case threadinbox.ThreadChosenMsg:
		return m.handleThreadChosen(msg)

	case threadinbox.FollowToggledMsg:
		return m, SetThreadFollowing(m.reqCtx(), m.client, msg.RootID, msg.Following)

	case threadinbox.ClosedMsg:
		return m, m.setFocus(FocusInput)

	case ProfileUpdatedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if msg.User != nil {
			m.applyMe(msg.User)
			cmds = append(cmds, m.setError(errProfileSaved))
		}
		return m, tea.Batch(cmds...)

	case DMCreatedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if msg.Channel != nil {
			m.addDMChannel(msg.Channel)
			cmds = append(cmds, m.selectChannel(msg.Channel))
			cmds = append(cmds, FetchDMChannels(m.reqCtx(), m.client))
		}
		return m, tea.Batch(cmds...)

	case chcreator.ChannelSubmittedMsg:
		if msg.Channel.Type == model.ChannelPrivate {
			m.pendingPrivateChannel = msg.Channel
			m.pendingMembers = nil
			cmd := m.setFocus(FocusDMPicker)
			m.dmPicker.OpenForMembers()
			return m, cmd
		}
		cmds = append(cmds, CreateChannel(m.reqCtx(), m.client, msg.Channel))
		return m, tea.Batch(cmds...)

	case dmpicker.CancelledMsg:
		return m.handleMemberPickCancelled(msg)

	case dmpicker.MembersPickedMsg:
		return m.handleMembersPicked(msg)

	case ChannelCreatedMsg:
		return m.handleChannelCreated(msg)

	case AllMembersAddedMsg:
		cmds = append(cmds, FetchChannelMembers(m.reqCtx(), m.client, msg.ChannelID))
		return m, tea.Batch(cmds...)

	case ChannelMemberAddedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		return m, nil

	case AllTagsLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.allTags = msg.Tags
		return m, nil

	case PostTagsLoadedMsg:
		return m.handlePostTagsLoaded(msg)

	case tagpicker.TagToggledMsg:
		if msg.Applied {
			cmds = append(cmds, AddTagToPostCmd(m.reqCtx(), m.client, msg.PostID, msg.TagID))
		} else {
			cmds = append(cmds, RemoveTagFromPostCmd(m.reqCtx(), m.client, msg.PostID, msg.TagID))
		}
		return m, tea.Batch(cmds...)

	case tagpicker.TagCreateRequestMsg:
		cmds = append(cmds, CreateTagAndApplyCmd(m.reqCtx(), m.client, msg.Name, msg.PostID))
		return m, tea.Batch(cmds...)

	case TagCreatedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if msg.Tag != nil {
			m.allTags = append(m.allTags, msg.Tag)
		}
		return m, nil

	case TagAddedToPostMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if msg.NewTag != nil && m.tagByName(msg.NewTag.Name) == nil {
			m.allTags = append(m.allTags, msg.NewTag)
		}
		m.tagPicker.AddApplied(msg.PostID, msg.NewTag)
		cmds = append(cmds, FetchPostTags(m.reqCtx(), m.client, msg.PostID))
		return m, tea.Batch(cmds...)

	case TagRemovedFromPostMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		cmds = append(cmds, FetchPostTags(m.reqCtx(), m.client, msg.PostID))
		return m, tea.Batch(cmds...)

	case skinpicker.SkinSelectedMsg:
		return m.handleThemeChosen(msg)

	case clipboardCopiedMsg:
		// OSC 52 gives no delivery confirmation, and several terminals ignore
		// it outright, so say what was sent rather than leaving the user to
		// guess whether anything happened.
		noun := "lines"
		if msg.lines == 1 {
			noun = "line"
		}
		m.viewport.ClearSelection()
		return m, m.setError(fmt.Errorf("copied %d %s to the clipboard", msg.lines, noun))

	case OlderPostsLoadedMsg:
		return m.handleOlderPosts(msg)

	case ChannelLeftMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		// The server also broadcasts user_removed, but that arrives only if
		// the socket is up; removing it here keeps leaving reliable.
		cmds = append(cmds, m.removeChannel(msg.ChannelID)...)
		return m, tea.Batch(cmds...)

	case PostPinnedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		// Shown now rather than on the WebSocket echo, which never comes
		// with the socket down; the echo then changes nothing.
		m.viewport.SetPinned(msg.PostID, msg.Pinned)
		return m, nil

	case PostEditedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if msg.Post != nil {
			m.viewport.UpdatePost(msg.Post)
			m.thread.UpdatePost(msg.Post)
		}
		return m, nil

	case PostDeletedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.viewport.RemovePost(msg.PostID)
		m.thread.RemoveReply(msg.PostID)
		return m, nil

	case PostsTagsLoadedMsg:
		return m.handlePostsTagsLoaded(msg)

	case ClearErrMsg:
		if msg.Seq == m.errSeq {
			m.err = nil
		}
		return m, nil

	case ErrMsg:
		if errors.Is(msg.Err, ws.ErrNotConnected) {
			m.wsConnected = false
		}
		return m, m.setError(msg.Err)
	}

	return m, nil
}
