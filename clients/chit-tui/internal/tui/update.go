package tui

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
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
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
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
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if m.expiredUserID != "" {
			return m.resumeAfterReLogin(msg.User)
		}
		m.applyMe(msg.User)
		return m, m.fetchMissingUsers()

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
			cmds = append(cmds, FetchChannels(m.client, t.ID))
		}
		return m, tea.Batch(cmds...)

	case ChannelsLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.channelsByTeam[msg.TeamID] = msg.Channels
		m.channels = m.flattenChannels()
		m.palette.SetChannels(m.channels)
		if m.activeTeam != nil && msg.TeamID == m.activeTeam.ID {
			// Auto-select the first channel only on the very first load, not
			// on later reloads (e.g. after navigating back to the team list).
			if !m.channelAutoSelected && m.activeChan == nil && len(msg.Channels) > 0 {
				cmds = append(cmds, m.selectChannel(msg.Channels[0]))
			}
		}
		return m, tea.Batch(cmds...)

	case PostsLoadedMsg:
		if m.activeChan == nil || msg.ChannelID != m.activeChan.ID {
			// The reader moved on while this was in flight; applying it
			// would show one channel's history under another's name.
			return m, nil
		}
		m.viewport.SetLoading(false)
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.viewport.SetPosts(msg.Posts.Order)
		m.historyPage = 0
		m.loadingOlder = false
		// A short first page means there is nothing older to ask for.
		m.historyExhausted = len(msg.Posts.Order) < historyPageSize
		m.postTags = make(map[string][]*model.Tag)
		return m, m.afterPostsLoaded(msg.Posts.Order)

	case PostCreatedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		// Show the message immediately rather than waiting for the WebSocket
		// echo. With the socket down the echo never arrives, so the input
		// cleared and the message simply vanished.
		if msg.Post != nil && msg.Post.Type != postTypeCommandResponse &&
			m.activeChan != nil && msg.Post.ChannelID == m.activeChan.ID &&
			!m.viewport.HasPost(msg.Post.ID) {
			m.viewport.AppendPost(msg.Post)
			m.resolvePostUsers([]*model.Post{msg.Post})
			if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
				cmds = append(cmds, fetchCmd)
			}
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

	case ThreadLoadedMsg:
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

	case CommandsLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.palette.SetCommands(mergeCommands(clientCommands(), msg.Commands))
		return m, nil

	case UsersLoadedMsg:
		if msg.Err != nil {
			// Worth asking again next time, unlike IDs the server answered.
			for _, id := range msg.Requested {
				delete(m.usersRequested, id)
			}
			return m, m.setError(msg.Err)
		}
		for _, u := range msg.Users {
			m.users[u.ID] = u
		}
		m.viewport.SetUsernames(m.usernameMap())
		m.thread.SetUsernames(m.usernameMap())
		m.resolveDMDisplayNames()
		return m, nil

	case ChannelMembersLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.channelMembers[msg.ChannelID] = msg.Members
		m.computeUnread(msg.ChannelID, msg.Members)
		m.computeMentions(msg.ChannelID, msg.Members)
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

	case WebSocketEventMsg:
		return m.handleWSEvent(msg)

	case viewport.PostSelectedMsg:
		m.cancelEdit()
		cmds = append(cmds, FetchThread(m.client, msg.Post.ID))
		m.mainPane = paneThread
		m.threadRootID = msg.Post.ID
		cmds = append(cmds, m.setFocus(FocusInput))
		m.resizeComponents()
		return m, tea.Batch(cmds...)

	case input.SendMsg:
		if m.activeChan == nil || m.me == nil {
			return m, nil
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
			cmds = append(cmds, CreatePost(m.client, reply))
			return m, tea.Batch(cmds...)
		}
		content, tagNames := tagpicker.StripHashtags(msg.Content)
		if content == "" && len(tagNames) > 0 {
			content = msg.Content
		}
		if m.editingPostID != "" {
			id := m.editingPostID
			m.editingPostID = ""
			cmds = append(cmds, EditPost(m.client, id, content))
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
		cmds = append(cmds, CreatePost(m.client, post, tagNames...))
		return m, tea.Batch(cmds...)

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
			cmds = append(cmds, CreateDMChannel(m.client, m.me.ID, msg.User.ID))
		}
		return m, tea.Batch(cmds...)

	case palette.UserQueryMsg:
		m.userQuery = msg.Term
		cmds = append(cmds, SearchUsersCmd(m.client, msg.Term))
		return m, tea.Batch(cmds...)

	case palette.DebounceMsg:
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd

	case palette.SearchSubmitMsg:
		term, tagNames := tagpicker.StripHashtags(msg.Term)
		var tagIDs []string
		for _, name := range tagNames {
			if t := m.tagByName(name); t != nil {
				tagIDs = append(tagIDs, t.ID)
			}
		}
		m.searchTerm = term
		m.searchWasGlobal = msg.Everywhere

		switch {
		case msg.Everywhere:
			cmds = append(cmds, SearchPostsEverywhere(m.client, term, tagIDs))
		case m.activeChan != nil:
			cmds = append(cmds, SearchPosts(m.client, m.activeChan.ID, term, tagIDs))
		}
		return m, tea.Batch(cmds...)

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
		// Jumping to the hit is the point of searching; previously this only
		// moved focus and left the reader wherever they already were.
		cmd := m.setFocus(FocusViewport)
		if msg.Post != nil {
			// A result from another channel needs that channel opened first;
			// the post is not in the loaded history until it is.
			if msg.Post.ChannelID != "" &&
				(m.activeChan == nil || msg.Post.ChannelID != m.activeChan.ID) {
				if ch := m.channelByID(msg.Post.ChannelID); ch != nil {
					return m, tea.Batch(cmd, m.selectChannel(ch), m.setError(errSearchHitElsewhere))
				}
			}
			m.viewport.SetSearchTerm(m.searchTerm)
			if !m.viewport.ScrollToPost(msg.Post.ID) {
				// The hit is older than the posts held in memory. Say so
				// rather than silently doing nothing.
				return m, tea.Batch(cmd, m.setError(errSearchHitNotLoaded))
			}
		}
		return m, cmd

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
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.dmChannels = msg.Channels
		m.palette.SetDMChannels(msg.Channels)
		m.resolveDMDisplayNames()
		// DM channels need their members for unread counts. This is still one
		// request each, but the DM list is small and bounded by conversations
		// the user actually has, unlike the channel list.
		// Every DM event reloads this list, so only conversations not seen
		// before are fetched.
		for _, ch := range msg.Channels {
			if _, have := m.channelMembers[ch.ID]; !have {
				cmds = append(cmds, FetchChannelMembers(m.client, ch.ID))
			}
		}
		if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
			cmds = append(cmds, fetchCmd)
		}
		return m, tea.Batch(cmds...)

	case dmpicker.SearchTriggeredMsg:
		m.userQuery = msg.Term
		cmds = append(cmds, SearchUsersCmd(m.client, msg.Term))
		return m, tea.Batch(cmds...)

	case UserSearchResultsMsg:
		if msg.Term != m.userQuery {
			return m, nil // superseded by a later keystroke
		}
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		filtered := msg.Users
		if m.me != nil {
			filtered = make([]*model.User, 0, len(msg.Users))
			for _, u := range msg.Users {
				if u.ID != m.me.ID {
					filtered = append(filtered, u)
				}
			}
		}
		// Both the palette ("@" mode) and the DM picker (member selection)
		// consume user searches; route to whichever is open.
		switch {
		case m.palette.Visible():
			m.palette.SetUsers(filtered)
		case m.dmPicker.Visible():
			m.dmPicker.SetResults(filtered)
		}
		return m, nil

	case ThreadsLoadedMsg:
		if msg.Err != nil {
			m.threadInbox.SetThreads(nil)
			return m, m.setError(msg.Err)
		}
		m.threadInbox.SetThreads(msg.Threads)
		return m, nil

	case ThreadFollowChangedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		return m, nil

	case threadinbox.ThreadChosenMsg:
		// Opening a thread means switching to its channel first: the thread
		// pane renders against the active channel, and the thread may well be
		// in one the user is not currently looking at.
		if ch := m.channelByID(msg.ChannelID); ch != nil && (m.activeChan == nil || m.activeChan.ID != ch.ID) {
			cmds = append(cmds, m.selectChannel(ch))
		}
		m.cancelEdit()
		cmds = append(cmds, FetchThread(m.client, msg.RootID))
		m.mainPane = paneThread
		m.threadRootID = msg.RootID
		cmds = append(cmds, m.setFocus(FocusThread))
		m.resizeComponents()
		if m.activeTeam != nil {
			cmds = append(cmds, MarkThreadRead(m.client, m.activeTeam.ID, msg.RootID))
		}
		return m, tea.Batch(cmds...)

	case threadinbox.FollowToggledMsg:
		if m.activeTeam == nil {
			return m, nil
		}
		return m, SetThreadFollowing(m.client, m.activeTeam.ID, msg.RootID, msg.Following)

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
			cmds = append(cmds, FetchDMChannels(m.client))
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
		cmds = append(cmds, CreateChannel(m.client, msg.Channel))
		return m, tea.Batch(cmds...)

	case dmpicker.CancelledMsg:
		// A group channel is nothing but its members, so an abandoned pick
		// leaves nothing to create — unlike a private channel, which was
		// already named and submitted before the picker opened.
		m.pendingGroupChannel = false
		// Dismissing the member picker skips member selection but still
		// creates the already-submitted private channel.
		if m.pendingPrivateChannel != nil {
			ch := m.pendingPrivateChannel
			m.pendingPrivateChannel = nil
			m.pendingMembers = nil
			cmds = append(cmds, CreateChannel(m.client, ch))
		}
		return m, tea.Batch(cmds...)

	case dmpicker.MembersPickedMsg:
		m.dmPicker.Close()

		if m.pendingGroupChannel {
			m.pendingGroupChannel = false
			if m.me == nil {
				return m, tea.Batch(cmds...)
			}
			// The server counts the creator among the members and rejects a
			// group that excludes them, so send the full membership rather
			// than just who was picked.
			ids := make([]string, 0, len(msg.Users)+1)
			ids = append(ids, m.me.ID)
			for _, u := range msg.Users {
				if u.ID != m.me.ID {
					ids = append(ids, u.ID)
				}
			}
			if len(ids) < minGroupChannelMembers {
				cmds = append(cmds, m.setError(errGroupTooSmall))
				return m, tea.Batch(cmds...)
			}
			cmds = append(cmds, CreateGroupChannel(m.client, ids))
			return m, tea.Batch(cmds...)
		}

		if m.pendingPrivateChannel != nil {
			for _, u := range msg.Users {
				m.pendingMembers = append(m.pendingMembers, u.ID)
			}
			cmds = append(cmds, CreateChannel(m.client, m.pendingPrivateChannel))
			m.pendingPrivateChannel = nil
		}
		return m, tea.Batch(cmds...)

	case ChannelCreatedMsg:
		if msg.Err != nil {
			cmd := m.setError(msg.Err)
			m.pendingMembers = nil
			return m, cmd
		}
		if msg.Channel != nil {
			teamID := msg.Channel.TeamID
			m.channelsByTeam[teamID] = append([]*model.Channel{msg.Channel}, m.channelsByTeam[teamID]...)
			m.channels = m.flattenChannels()
			m.palette.SetChannels(m.channels)
			cmds = append(cmds, m.selectChannel(msg.Channel))
			cmds = append(cmds, FetchChannelMembers(m.client, msg.Channel.ID))
			if len(m.pendingMembers) > 0 {
				members := m.pendingMembers
				m.pendingMembers = nil
				cmds = append(cmds, AddChannelMembersCmd(m.client, msg.Channel.ID, members))
			}
		}
		return m, tea.Batch(cmds...)

	case AllMembersAddedMsg:
		cmds = append(cmds, FetchChannelMembers(m.client, msg.ChannelID))
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
		if msg.Err != nil {
			// Tags are decoration, but silently never appearing looks like
			// the post has none.
			return m, m.setError(fmt.Errorf("could not load tags: %w", msg.Err))
		}
		m.postTags[msg.PostID] = msg.Tags
		var names []string
		for _, t := range msg.Tags {
			names = append(names, t.Name)
		}
		m.viewport.SetPostTags(msg.PostID, names)
		// The thread pane renders the same posts, so it needs the tags too;
		// otherwise a post shows its tags in the channel and loses them the
		// moment it is opened as a thread.
		m.thread.SetPostTags(m.postTags)
		return m, nil

	case tagpicker.TagToggledMsg:
		if msg.Applied {
			cmds = append(cmds, AddTagToPostCmd(m.client, msg.PostID, msg.TagID))
		} else {
			cmds = append(cmds, RemoveTagFromPostCmd(m.client, msg.PostID, msg.TagID))
		}
		return m, tea.Batch(cmds...)

	case tagpicker.TagCreateRequestMsg:
		cmds = append(cmds, CreateTagAndApplyCmd(m.client, msg.Name, msg.PostID))
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
		cmds = append(cmds, FetchPostTags(m.client, msg.PostID))
		return m, tea.Batch(cmds...)

	case TagRemovedFromPostMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		cmds = append(cmds, FetchPostTags(m.client, msg.PostID))
		return m, tea.Batch(cmds...)

	case skinpicker.SkinSelectedMsg:
		t, _, err := theme.ResolveNamed(msg.Name, config.ThemesDir())
		if err != nil {
			// The name came from a list this component built, so a failure
			// here means the file changed underneath us.
			errCmd := m.setError(fmt.Errorf("load theme %q: %w", msg.Name, err))
			return m, tea.Batch(m.setFocus(FocusInput), errCmd)
		}

		// Persist so the choice survives a restart. Failing to write is worth
		// surfacing but must not undo the theme change for this session.
		var saveCmd tea.Cmd
		if err := config.SaveThemeSetting(m.themeSetting, msg.Name); err != nil {
			saveCmd = m.setError(fmt.Errorf("theme applied but not saved: %w", err))
		}

		newStyles := styles.New(t)
		m.styles = newStyles
		m.viewport.SetStyles(newStyles)
		m.input.SetStyles(newStyles)
		m.thread.SetStyles(newStyles)
		m.mention.SetStyles(newStyles)
		m.actionBar.SetStyles(newStyles)
		for _, o := range m.overlays() {
			o.setStyles(newStyles)
		}
		return m, tea.Batch(m.setFocus(FocusInput), saveCmd)

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
		if m.activeChan == nil || msg.ChannelID != m.activeChan.ID || msg.Page != m.historyPage+1 {
			// The reader moved on, or the history was reloaded, while this
			// was in flight.
			return m, nil
		}
		m.loadingOlder = false
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		older := msg.Posts.Order
		if len(older) < historyPageSize {
			m.historyExhausted = true
		}
		if len(older) == 0 {
			return m, nil
		}
		m.historyPage = msg.Page
		m.viewport.PrependPosts(older)
		return m, m.afterPostsLoaded(older)

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
		// The server broadcasts post_pinned with the full post, which
		// updates the badge; nothing more is needed here.
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
		if msg.Err != nil {
			// Tags are decoration; a failure here should not disturb the
			// channel, but it should not vanish silently either.
			return m, m.setError(msg.Err)
		}
		byPost := make(map[string][]string, len(msg.Tags))
		for postID, tags := range msg.Tags {
			m.postTags[postID] = tags
			names := make([]string, 0, len(tags))
			for _, t := range tags {
				names = append(names, t.Name)
			}
			byPost[postID] = names
		}
		m.viewport.SetPostsTags(byPost)
		m.thread.SetPostTags(m.postTags)
		return m, nil

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
