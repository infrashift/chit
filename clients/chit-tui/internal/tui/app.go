package tui

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/auth"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/actionbar"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/chcreator"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/help"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/login"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/mention"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/thread"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/viewport"
	"github.com/infrashift/chit/clients/chit-tui/internal/ws"
)

// FocusArea defines which component has focus.
type FocusArea int

const (
	FocusViewport FocusArea = iota
	FocusInput
	FocusThread
	FocusPalette
	FocusDMPicker
	FocusSkinPicker
	FocusChCreator
	FocusTagPicker
)

// mainPane selects what fills the top content pane.
type mainPane int

const (
	paneChannel mainPane = iota
	paneThread
)

// AppState tracks whether we are on the login screen or the main app.
type AppState int

const (
	AppStateLogin AppState = iota
	AppStateReLogin
	AppStateRunning
)

// Model is the root TUI model.
type Model struct {
	cfg                   *config.Config
	client                api.ChitClient
	wsClient              ws.WSClient
	viewport              viewport.Model
	input                 input.Model
	thread                thread.Model
	actionBar             actionbar.Model
	palette               palette.Model
	help                  help.Model
	mention               mention.Model
	dmPicker              dmpicker.Model
	skinPicker            skinpicker.Model
	chCreator             chcreator.Model
	tagPicker             tagpicker.Model
	loginModel            login.Model
	me                    *model.User
	activeTeam            *model.Team
	activeChan            *model.Channel
	teams                 []*model.Team
	channels              []*model.Channel
	channelsByTeam        map[string][]*model.Channel
	dmChannels            []*model.Channel
	unread                map[string]int64
	mentions              map[string]int64
	dmDisplayNames        map[string]string
	users                 map[string]*model.User
	channelMembers        map[string][]*model.ChannelMember
	threadCounts          map[string]int
	allTags               []*model.Tag
	postTags              map[string][]*model.Tag
	pendingPostTags       []string
	pendingPrivateChannel *model.Channel
	pendingMembers        []string
	appState              AppState
	tokenStore            *auth.TokenStore
	kratosClient          *auth.KratosClient
	sessionStore          *auth.SessionStore
	focus                 FocusArea
	mainPane              mainPane
	channelAutoSelected   bool
	wsConnected           bool
	lastWSSeq             int64
	keys                  KeyMap
	styles                styles.Styles
	width                 int
	height                int
	err                   error
	errSeq                uint64
}

// NewModel creates the root model.
func NewModel(cfg *config.Config, client api.ChitClient, wsClient ws.WSClient, s styles.Styles, tokenStore *auth.TokenStore, kratosClient *auth.KratosClient, sessionStore *auth.SessionStore) Model {
	state := AppStateRunning
	if tokenStore != nil && tokenStore.Get() == "" {
		state = AppStateLogin
	}
	if sessionStore == nil {
		sessionStore = auth.NewSessionStore("")
	}

	m := Model{
		cfg:            cfg,
		client:         client,
		wsClient:       wsClient,
		viewport:       viewport.New(s),
		input:          input.New(s),
		actionBar:      actionbar.New(s),
		thread:         thread.New(s),
		palette:        palette.New(s),
		help:           help.New(s),
		mention:        mention.New(s),
		dmPicker:       dmpicker.New(s),
		skinPicker:     skinpicker.New(s),
		chCreator:      chcreator.New(s),
		tagPicker:      tagpicker.New(s),
		loginModel:     login.New(s, kratosClient),
		users:          make(map[string]*model.User),
		channelsByTeam: make(map[string][]*model.Channel),
		unread:         make(map[string]int64),
		mentions:       make(map[string]int64),
		dmDisplayNames: make(map[string]string),
		channelMembers: make(map[string][]*model.ChannelMember),
		threadCounts:   make(map[string]int),
		postTags:       make(map[string][]*model.Tag),
		appState:       state,
		tokenStore:     tokenStore,
		kratosClient:   kratosClient,
		sessionStore:   sessionStore,
		focus:          FocusInput,
		keys:           DefaultKeyMap(),
		styles:         s,
	}
	// The palette shares the root's badge and DM-name maps by reference, so
	// root-side updates are visible without further plumbing.
	m.palette.SetCounts(m.unread, m.mentions)
	m.palette.SetDMDisplayNames(m.dmDisplayNames)
	_ = m.input.Focus()
	return m
}

// setError sets the error and returns a command to auto-clear it after 10 seconds.
func (m *Model) setError(err error) tea.Cmd {
	m.errSeq++
	m.err = err
	seq := m.errSeq
	return tea.Tick(10*time.Second, func(_ time.Time) tea.Msg {
		return ClearErrMsg{Seq: seq}
	})
}

// Init returns the initial commands.
func (m Model) Init() tea.Cmd {
	if m.appState == AppStateLogin || m.appState == AppStateReLogin {
		return nil
	}
	return m.initRunning()
}

func (m Model) initRunning() tea.Cmd {
	cmds := []tea.Cmd{
		FetchMe(m.client),
		FetchTeams(m.client),
		FetchCommands(m.client),
		FetchDMChannels(m.client),
		FetchAllTags(m.client),
	}
	if m.wsClient != nil {
		cmds = append(cmds, func() tea.Msg {
			if err := m.wsClient.Connect(); err != nil {
				return ErrMsg{Err: err}
			}
			return WSConnectedMsg{}
		})
	}
	return tea.Batch(cmds...)
}

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

	switch msg := msg.(type) {
	case login.LoginSuccessMsg:
		return m.handleLoginSuccess(msg)

	case login.LoginErrorMsg:
		m.loginModel, _ = m.loginModel.Update(msg)
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		// Global keybindings (Ctrl+C always quits)
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		// A visible overlay intercepts all keys. If it self-closes (Esc or a
		// selection), restore focus to a live component and run any
		// close-time behavior.
		for _, o := range m.overlays() {
			if !o.visible() {
				continue
			}
			cmd := o.update(msg)
			if !o.visible() {
				if o.closeFocus != focusKeep {
					return m, tea.Batch(cmd, m.setFocus(o.closeFocus))
				}
				return m, cmd
			}
			return m, cmd
		}

		// Intercept keys when mention popup is visible
		if m.mention.Visible() {
			switch msg.Type {
			case tea.KeyUp, tea.KeyDown, tea.KeyEnter, tea.KeyTab, tea.KeyEscape:
				var cmd tea.Cmd
				m.mention, cmd = m.mention.Update(msg)
				return m, cmd
			}
		}

		if key.Matches(msg, m.keys.NewDM) {
			return m, m.openPalette("@")
		}

		if key.Matches(msg, m.keys.NewChannel) && m.focus != FocusChCreator && m.activeTeam != nil {
			return m, m.openChCreator()
		}

		if key.Matches(msg, m.keys.CmdPalette) {
			return m, m.openPalette("")
		}

		if key.Matches(msg, m.keys.Search) {
			return m, m.openPalette("?")
		}

		// "?" is a printable key: only open help when not typing in a text box.
		if key.Matches(msg, m.keys.Help) && (m.focus == FocusViewport || m.focus == FocusThread) {
			m.help.Open()
			return m, nil
		}

		if key.Matches(msg, m.keys.Escape) {
			if m.mainPane == paneThread {
				return m, m.closeThread()
			}
			if m.focus == FocusInput {
				return m, m.setFocus(FocusViewport)
			}
		}

		if key.Matches(msg, m.keys.TagPicker) && m.focus == FocusViewport {
			sel := m.viewport.SelectedPost()
			if sel != nil {
				cmd := m.setFocus(FocusTagPicker)
				m.tagPicker.Open(sel.ID, m.allTags, m.postTags[sel.ID])
				return m, cmd
			}
		}

		if key.Matches(msg, m.keys.Tab) {
			return m, m.cycleFocus(1)
		}
		if key.Matches(msg, m.keys.ShiftTab) {
			return m, m.cycleFocus(-1)
		}

		// Delegate to focused component
		return m.delegateKey(msg)

	case UserLoadedMsg:
		if msg.Err != nil {
			if api.IsUnauthorized(msg.Err) {
				return m.handleAuthExpired()
			}
			return m, m.setError(msg.Err)
		}
		m.me = msg.User
		m.users[msg.User.ID] = msg.User
		m.viewport.SetCurrentUsername(msg.User.Username)
		m.thread.SetCurrentUsername(msg.User.Username)
		m.resolveDMDisplayNames()
		return m, m.fetchMissingUsers()

	case TeamsLoadedMsg:
		if msg.Err != nil {
			if api.IsUnauthorized(msg.Err) {
				return m.handleAuthExpired()
			}
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
			if api.IsUnauthorized(msg.Err) {
				return m.handleAuthExpired()
			}
			return m, m.setError(msg.Err)
		}
		m.channelsByTeam[msg.TeamID] = msg.Channels
		m.channels = m.flattenChannels()
		m.palette.SetChannels(m.channels)
		if m.activeTeam != nil && msg.TeamID == m.activeTeam.ID {
			// Auto-select the first channel only on the very first load, not
			// on later reloads (e.g. after navigating back to the team list).
			if !m.channelAutoSelected && m.activeChan == nil && len(msg.Channels) > 0 {
				m.channelAutoSelected = true
				m.activeChan = msg.Channels[0]
				cmds = append(cmds, FetchPosts(m.client, msg.Channels[0].ID, 0, 60))
				cmds = append(cmds, ViewChannel(m.client, msg.Channels[0].ID))
			}
		}
		for _, ch := range msg.Channels {
			cmds = append(cmds, FetchChannelMembers(m.client, ch.ID))
		}
		return m, tea.Batch(cmds...)

	case PostsLoadedMsg:
		if msg.Err != nil {
			if api.IsUnauthorized(msg.Err) {
				return m.handleAuthExpired()
			}
			return m, m.setError(msg.Err)
		}
		m.viewport.SetPosts(msg.Posts.Order)
		m.resolvePostUsers(msg.Posts.Order)
		m.postTags = make(map[string][]*model.Tag)
		for _, p := range msg.Posts.Order {
			cmds = append(cmds, FetchPostTags(m.client, p.ID))
		}
		if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
			cmds = append(cmds, fetchCmd)
		}
		return m, tea.Batch(cmds...)

	case PostCreatedMsg:
		if msg.Err != nil {
			if api.IsUnauthorized(msg.Err) {
				return m.handleAuthExpired()
			}
			cmd := m.setError(msg.Err)
			m.pendingPostTags = nil
			return m, cmd
		}
		if msg.Post != nil && len(m.pendingPostTags) > 0 {
			for _, tagName := range m.pendingPostTags {
				found := false
				for _, t := range m.allTags {
					if strings.EqualFold(t.Name, tagName) {
						cmds = append(cmds, AddTagToPostCmd(m.client, msg.Post.ID, t.ID))
						found = true
						break
					}
				}
				if !found {
					cmds = append(cmds, CreateTagAndApplyCmd(m.client, tagName, msg.Post.ID))
				}
			}
			m.pendingPostTags = nil
			return m, tea.Batch(cmds...)
		}
		m.pendingPostTags = nil
		return m, nil

	case ThreadLoadedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
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
		m.palette.SetCommands(msg.Commands)
		return m, nil

	case UsersLoadedMsg:
		if msg.Err != nil {
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
		if msg.Err == nil {
			m.setUnread(msg.ChannelID, 0)
			m.setMention(msg.ChannelID, 0)
		}
		return m, nil

	case WSConnectedMsg:
		m.wsConnected = true
		return m, ListenWebSocket(m.wsClient)

	case WebSocketEventMsg:
		return m.handleWSEvent(msg)

	case viewport.PostSelectedMsg:
		cmds = append(cmds, FetchThread(m.client, msg.Post.ID))
		m.mainPane = paneThread
		cmds = append(cmds, m.setFocus(FocusInput))
		m.resizeComponents()
		return m, tea.Batch(cmds...)

	case input.SendMsg:
		if m.activeChan == nil || m.me == nil {
			return m, nil
		}
		// In the thread pane the input composes a reply to the thread root.
		if m.mainPane == paneThread && m.thread.RootPost() != nil {
			reply := &model.Post{
				ChannelID: m.activeChan.ID,
				UserID:    m.me.ID,
				RootID:    m.thread.RootPost().ID,
				Content:   msg.Content,
			}
			cmds = append(cmds, CreatePost(m.client, reply))
			return m, tea.Batch(cmds...)
		}
		content, tagNames := tagpicker.StripHashtags(msg.Content)
		if content == "" && len(tagNames) > 0 {
			content = msg.Content
		}
		m.pendingPostTags = tagNames
		post := &model.Post{
			ChannelID: m.activeChan.ID,
			UserID:    m.me.ID,
			Content:   content,
		}
		cmds = append(cmds, CreatePost(m.client, post))
		return m, tea.Batch(cmds...)

	case input.SlashTriggerMsg:
		trimmed := strings.TrimSpace(msg.Input)
		if trimmed == "/skin" {
			m.skinPicker.SetSkins(theme.ListAvailable())
			cmd := m.setFocus(FocusSkinPicker)
			m.skinPicker.Open()
			return m, cmd
		}
		if trimmed == "/logout" {
			return m.handleLogout()
		}
		return m, m.openPalette("/")

	case palette.ChannelChosenMsg:
		cmds = append(cmds, m.selectChannel(msg.Channel))
		cmds = append(cmds, m.setFocus(FocusInput))
		return m, tea.Batch(cmds...)

	case palette.CommandChosenMsg:
		cmd := m.setFocus(FocusInput)
		return m, cmd

	case palette.UserChosenMsg:
		if m.me != nil {
			cmds = append(cmds, CreateDMChannel(m.client, m.me.ID, msg.User.ID))
		}
		return m, tea.Batch(cmds...)

	case palette.UserQueryMsg:
		cmds = append(cmds, SearchUsersCmd(m.client, msg.Term))
		return m, tea.Batch(cmds...)

	case palette.DebounceMsg:
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		return m, cmd

	case palette.SearchSubmitMsg:
		if m.activeChan != nil {
			term, tagNames := tagpicker.StripHashtags(msg.Term)
			var tagIDs []string
			for _, name := range tagNames {
				for _, t := range m.allTags {
					if strings.EqualFold(t.Name, name) {
						tagIDs = append(tagIDs, t.ID)
						break
					}
				}
			}
			cmds = append(cmds, SearchPosts(m.client, m.activeChan.ID, term, tagIDs))
		}
		return m, tea.Batch(cmds...)

	case SearchResultsMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if msg.Posts != nil {
			m.palette.SetUsernames(m.usernameMap())
			m.palette.SetSearchResults(msg.Posts.Order)
		}
		return m, nil

	case palette.PostChosenMsg:
		cmd := m.setFocus(FocusViewport)
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
		// Fetch members for DM channels (for unread counts)
		for _, ch := range msg.Channels {
			cmds = append(cmds, FetchChannelMembers(m.client, ch.ID))
		}
		if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
			cmds = append(cmds, fetchCmd)
		}
		return m, tea.Batch(cmds...)

	case dmpicker.SearchTriggeredMsg:
		cmds = append(cmds, SearchUsersCmd(m.client, msg.Term))
		return m, tea.Batch(cmds...)

	case UserSearchResultsMsg:
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
		if m.palette.Visible() {
			m.palette.SetUsers(filtered)
			return m, nil
		}
		m.dmPicker.SetResults(filtered)
		return m, nil

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
			return m, nil
		}
		m.postTags[msg.PostID] = msg.Tags
		var names []string
		for _, t := range msg.Tags {
			names = append(names, t.Name)
		}
		m.viewport.SetPostTags(msg.PostID, names)
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
		cmds = append(cmds, FetchPostTags(m.client, msg.PostID))
		cmds = append(cmds, FetchAllTags(m.client))
		return m, tea.Batch(cmds...)

	case TagRemovedFromPostMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		cmds = append(cmds, FetchPostTags(m.client, msg.PostID))
		return m, tea.Batch(cmds...)

	case skinpicker.SkinSelectedMsg:
		t := theme.LoadNamed(msg.Name)
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
		cmd := m.setFocus(FocusInput)
		return m, cmd

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

// View renders the full UI.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.appState == AppStateLogin || m.appState == AppStateReLogin {
		return m.loginModel.View()
	}

	top := m.viewport.View()
	if m.mainPane == paneThread {
		top = m.thread.View()
	}
	layout := lipgloss.JoinVertical(lipgloss.Left,
		top,
		m.input.View(),
	)

	// Composite the centered floating overlays.
	for _, o := range m.overlays() {
		if !o.visible() {
			continue
		}
		view := o.view()
		x := (m.width - lipgloss.Width(view)) / 2
		layout = placeOverlay(x, overlayY, view, layout)
	}

	// The mention autocomplete is anchored above the input box instead.
	if m.mention.Visible() {
		mentionView := m.mention.View()
		mentionHeight := lipgloss.Height(mentionView)
		inputHeight := 5
		mentionY := m.height - inputHeight - mentionHeight - 1
		if mentionY < 0 {
			mentionY = 0
		}
		layout = placeOverlay(1, mentionY, mentionView, layout)
	}

	// Action/status bar (m is a value receiver, so these mutations are local
	// to this render).
	m.syncActionBar()
	return lipgloss.JoinVertical(lipgloss.Left, layout, m.actionBar.View())
}

// overlayY is the row where centered floating overlays are anchored.
const overlayY = 2

// focusKeep, used as an overlayRef closeFocus, leaves focus untouched when
// the overlay closes (for overlays that never take focus, like help).
const focusKeep FocusArea = -1

// overlayRef is one floating overlay's hooks: visibility, key routing,
// rendering, and lifecycle plumbing. Every overlay consumer (key intercepts,
// View compositing, blur/resize/restyle, mouse hit-testing) iterates the
// single overlays() table, so adding an overlay means adding one entry here.
type overlayRef struct {
	visible    func() bool
	update     func(tea.KeyMsg) tea.Cmd
	view       func() string
	blur       func()
	setSize    func(w, h int)
	setStyles  func(styles.Styles)
	closeFocus FocusArea // focus target after the overlay self-closes
}

// overlays lists the centered floating overlays in z-order (later entries
// render on top; earlier entries win key interception, though only one
// overlay is ever open at a time).
func (m *Model) overlays() []overlayRef {
	return []overlayRef{
		{
			visible: func() bool { return m.dmPicker.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.dmPicker, cmd = m.dmPicker.Update(msg)
				return cmd
			},
			view:       func() string { return m.dmPicker.View() },
			blur:       m.dmPicker.Blur,
			setSize:    m.dmPicker.SetSize,
			setStyles:  m.dmPicker.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.skinPicker.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.skinPicker, cmd = m.skinPicker.Update(msg)
				return cmd
			},
			view:       func() string { return m.skinPicker.View() },
			blur:       m.skinPicker.Blur,
			setSize:    m.skinPicker.SetSize,
			setStyles:  m.skinPicker.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.chCreator.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.chCreator, cmd = m.chCreator.Update(msg)
				return cmd
			},
			view:       func() string { return m.chCreator.View() },
			blur:       m.chCreator.Blur,
			setSize:    m.chCreator.SetSize,
			setStyles:  m.chCreator.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.tagPicker.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.tagPicker, cmd = m.tagPicker.Update(msg)
				return cmd
			},
			view:       func() string { return m.tagPicker.View() },
			blur:       m.tagPicker.Blur,
			setSize:    m.tagPicker.SetSize,
			setStyles:  m.tagPicker.SetStyles,
			closeFocus: FocusViewport,
		},
		{
			visible: func() bool { return m.palette.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.palette, cmd = m.palette.Update(msg)
				return cmd
			},
			view:       func() string { return m.palette.View() },
			blur:       m.palette.Blur,
			setSize:    m.palette.SetSize,
			setStyles:  m.palette.SetStyles,
			closeFocus: FocusInput,
		},
		{
			visible: func() bool { return m.help.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.help, cmd = m.help.Update(msg)
				return cmd
			},
			view:       func() string { return m.help.View() },
			blur:       func() {},
			setSize:    m.help.SetSize,
			setStyles:  m.help.SetStyles,
			closeFocus: focusKeep, // help never takes focus
		},
	}
}

// syncActionBar pushes the current model state into the action bar before it
// is rendered or hit-tested.
func (m *Model) syncActionBar() {
	team := ""
	if m.activeTeam != nil {
		team = m.activeTeam.DisplayName
	}
	user := ""
	if m.me != nil {
		user = m.me.Username
	}
	errStr := ""
	if m.err != nil {
		errStr = m.err.Error()
	}
	m.actionBar.SetContext(team, m.activeChannelDisplayName(), user)
	m.actionBar.SetError(errStr)
	m.actionBar.SetConnected(m.wsConnected)
	m.actionBar.SetThreadOpen(m.mainPane == paneThread)
}

func (m *Model) setFocus(area FocusArea) tea.Cmd {
	m.viewport.Blur()
	m.input.Blur()
	m.thread.Blur()
	for _, o := range m.overlays() {
		o.blur()
	}

	m.focus = area
	switch area {
	case FocusViewport:
		m.viewport.Focus()
	case FocusInput:
		return m.input.Focus()
	case FocusThread:
		m.thread.Focus()
	case FocusPalette:
		m.palette.Focus()
	case FocusDMPicker:
		m.dmPicker.Focus()
	case FocusSkinPicker:
		m.skinPicker.Focus()
	case FocusChCreator:
		m.chCreator.Focus()
	case FocusTagPicker:
		m.tagPicker.Focus()
	}
	return nil
}

// selectChannel makes ch the active channel: marks the previous channel as
// viewed, loads posts, resets thread state, and derives the active team.
func (m *Model) selectChannel(ch *model.Channel) tea.Cmd {
	var cmds []tea.Cmd
	if m.activeChan != nil && m.activeChan.ID != ch.ID {
		cmds = append(cmds, ViewChannel(m.client, m.activeChan.ID))
	}
	m.activeChan = ch
	m.channelAutoSelected = true
	// DM/group channels have no team; keep the last active team then.
	if ch.TeamID != "" {
		if t := m.teamByID(ch.TeamID); t != nil {
			m.activeTeam = t
		}
	}
	cmds = append(cmds, FetchPosts(m.client, ch.ID, 0, 60))
	cmds = append(cmds, ViewChannel(m.client, ch.ID))
	m.mainPane = paneChannel
	m.thread.Clear()
	m.threadCounts = make(map[string]int)
	m.viewport.SetThreadCounts(m.threadCounts)
	m.resizeComponents()
	return tea.Batch(cmds...)
}

// addDMChannel prepends a DM/group channel to the list if it is not already
// present and refreshes derived display names.
func (m *Model) addDMChannel(ch *model.Channel) {
	for _, existing := range m.dmChannels {
		if existing.ID == ch.ID {
			m.resolveDMDisplayNames()
			return
		}
	}
	m.dmChannels = append([]*model.Channel{ch}, m.dmChannels...)
	m.palette.SetDMChannels(m.dmChannels)
	m.resolveDMDisplayNames()
}

// flattenChannels merges the per-team channel lists into one slice, in team
// order. Buckets are only ever keyed by known team IDs.
func (m Model) flattenChannels() []*model.Channel {
	var flat []*model.Channel
	for _, t := range m.teams {
		flat = append(flat, m.channelsByTeam[t.ID]...)
	}
	return flat
}

// UnreadCount returns the unread count for a channel.
func (m Model) UnreadCount(channelID string) int64 { return m.unread[channelID] }

// MentionCount returns the mention count for a channel.
func (m Model) MentionCount(channelID string) int64 { return m.mentions[channelID] }

// teamByID returns the team with the given ID, or nil.
func (m Model) teamByID(id string) *model.Team {
	for _, t := range m.teams {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// setUnread updates the unread count for a channel.
func (m *Model) setUnread(channelID string, count int64) {
	m.unread[channelID] = count
}

// setMention updates the mention count for a channel.
func (m *Model) setMention(channelID string, count int64) {
	m.mentions[channelID] = count
}

// closeThread returns from the thread pane to the channel view.
func (m *Model) closeThread() tea.Cmd {
	m.mainPane = paneChannel
	m.thread.Clear()
	cmd := m.setFocus(FocusViewport)
	m.resizeComponents()
	return cmd
}

// openChCreator opens the channel creator overlay for the active team.
func (m *Model) openChCreator() tea.Cmd {
	if m.activeTeam == nil {
		return nil
	}
	cmd := m.setFocus(FocusChCreator)
	m.chCreator.Open(m.activeTeam.ID)
	return cmd
}

// openPalette opens the unified palette with the query pre-filled to select
// a mode ("" channels, "@" people, "/" commands, "?" search).
func (m *Model) openPalette(prefix string) tea.Cmd {
	cmd := m.setFocus(FocusPalette)
	m.palette.SetActiveChannel(m.activeChannelDisplayName())
	m.palette.Open(prefix)
	return cmd
}

// paletteOrigin returns the screen position of the palette overlay. It is the
// single source of truth shared by View and mouse hit-testing.
func (m Model) paletteOrigin(view string) (int, int) {
	return (m.width - lipgloss.Width(view)) / 2, overlayY
}

func (m *Model) cycleFocus(dir int) tea.Cmd {
	areas := []FocusArea{FocusViewport, FocusInput}
	if m.mainPane == paneThread {
		areas = []FocusArea{FocusThread, FocusInput}
	}

	current := 0
	for i, a := range areas {
		if a == m.focus {
			current = i
			break
		}
	}

	next := (current + dir + len(areas)) % len(areas)
	return m.setFocus(areas[next])
}

func (m *Model) resizeComponents() {
	inputHeight := 5
	vpHeight := m.height - inputHeight - 1 // -1 for the action bar

	m.viewport.SetSize(m.width, vpHeight)
	m.input.SetSize(m.width, inputHeight)
	m.actionBar.SetSize(m.width)
	m.thread.SetSize(m.width, vpHeight)
	for _, o := range m.overlays() {
		o.setSize(m.width, m.height)
	}
}

// delegateKey routes a key to the focused main component. Overlay focus
// areas never reach here: a visible overlay intercepts keys earlier in
// Update, and closing one restores focus to a main component.
func (m Model) delegateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case FocusViewport:
		m.viewport, cmd = m.viewport.Update(msg)
	case FocusInput:
		m.input, cmd = m.input.Update(msg)
	case FocusThread:
		m.thread, cmd = m.thread.Update(msg)
	}
	return m, cmd
}

func (m Model) handleWSEvent(msg WebSocketEventMsg) (tea.Model, tea.Cmd) {
	evt := msg.Event
	if evt.Sequence > 0 && evt.Sequence <= m.lastWSSeq {
		return m, ListenWebSocket(m.wsClient)
	}
	if evt.Sequence > m.lastWSSeq {
		m.lastWSSeq = evt.Sequence
	}

	var cmds []tea.Cmd
	cmds = append(cmds, ListenWebSocket(m.wsClient))

	switch evt.Event {
	case model.WebSocketEventPosted:
		data, err := json.Marshal(evt.Data)
		if err != nil {
			return m, tea.Batch(cmds...)
		}
		var p model.Post
		if err := json.Unmarshal(data, &p); err != nil {
			return m, tea.Batch(cmds...)
		}
		if m.activeChan != nil && p.ChannelID == m.activeChan.ID {
			m.viewport.AppendPost(&p)
			if p.RootID != "" {
				m.threadCounts[p.RootID]++
				m.viewport.SetThreadCounts(m.threadCounts)
				if m.mainPane == paneThread && m.thread.RootPost() != nil && m.thread.RootPost().ID == p.RootID {
					m.thread.AppendReply(&p)
				}
			}
			m.resolvePostUsers([]*model.Post{&p})
			if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
				cmds = append(cmds, fetchCmd)
			}
		} else {
			m.setUnread(p.ChannelID, m.unread[p.ChannelID]+1)
		}

	case model.WebSocketEventThreadUpdated:
		if threadData, ok := evt.Data["thread"]; ok {
			data, err := json.Marshal(threadData)
			if err == nil {
				var t model.Thread
				if json.Unmarshal(data, &t) == nil {
					m.threadCounts[t.PostID] = t.ReplyCount
					m.viewport.SetThreadCounts(m.threadCounts)
				}
			}
		}
		if postData, ok := evt.Data["post"]; ok {
			data, err := json.Marshal(postData)
			if err == nil {
				var p model.Post
				if json.Unmarshal(data, &p) == nil {
					if p.RootID != "" && m.mainPane == paneThread && m.thread.RootPost() != nil && m.thread.RootPost().ID == p.RootID {
						m.thread.AppendReply(&p)
					}
				}
			}
		}

	case model.WebSocketEventMentioned:
		if evt.Broadcast != nil && evt.Broadcast.ChannelID != "" {
			chanID := evt.Broadcast.ChannelID
			m.setMention(chanID, m.mentions[chanID]+1)
		}

	case model.WebSocketEventUserAdded:
		if m.me != nil {
			if uid, ok := evt.Data["user_id"]; ok {
				if userID, ok := uid.(string); ok && userID == m.me.ID {
					if m.activeTeam != nil {
						cmds = append(cmds, FetchChannels(m.client, m.activeTeam.ID))
					}
				}
			}
		}

	case model.WebSocketEventChannelCreated:
		if typeVal, ok := evt.Data["type"]; ok {
			if t, ok := typeVal.(string); ok {
				switch t {
				case model.ChannelDirect, model.ChannelGroup:
					cmds = append(cmds, FetchDMChannels(m.client))
				case model.ChannelOpen, model.ChannelPrivate:
					if m.activeTeam != nil {
						if teamID, ok := evt.Data["team_id"]; ok {
							if tid, ok := teamID.(string); ok && tid == m.activeTeam.ID {
								cmds = append(cmds, FetchChannels(m.client, m.activeTeam.ID))
							}
						}
					}
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
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

func (m Model) buildMentionEntries() []mention.MentionEntry {
	var entries []mention.MentionEntry
	// Add special mentions
	entries = append(entries,
		mention.MentionEntry{Username: "all", Special: true},
		mention.MentionEntry{Username: "channel", Special: true},
		mention.MentionEntry{Username: "here", Special: true},
	)
	// Add channel members
	if m.activeChan != nil {
		members := m.channelMembers[m.activeChan.ID]
		for _, mem := range members {
			if m.me != nil && mem.UserID == m.me.ID {
				continue // skip self
			}
			u := m.users[mem.UserID]
			if u != nil {
				entries = append(entries, mention.MentionEntry{
					Username:    u.Username,
					DisplayName: u.DisplayName,
					UserID:      u.ID,
				})
			}
		}
	}
	return entries
}

func (m *Model) resolveDMDisplayNames() {
	if m.me == nil {
		return
	}
	for _, ch := range m.dmChannels {
		switch ch.Type {
		case model.ChannelDirect:
			parts := strings.Split(ch.Name, "__")
			if len(parts) != 2 {
				continue
			}
			otherID := parts[0]
			if otherID == m.me.ID {
				otherID = parts[1]
			}
			if u, ok := m.users[otherID]; ok && u != nil {
				name := u.Username
				if u.DisplayName != "" {
					name = u.DisplayName
				}
				m.dmDisplayNames[ch.ID] = name
			} else {
				if _, exists := m.users[otherID]; !exists {
					m.users[otherID] = nil
				}
			}

		case model.ChannelGroup:
			parts := strings.Split(ch.Name, "__")
			var names []string
			for _, uid := range parts {
				if uid == m.me.ID {
					continue
				}
				if u, ok := m.users[uid]; ok && u != nil {
					names = append(names, u.Username)
				} else {
					if _, exists := m.users[uid]; !exists {
						m.users[uid] = nil
					}
				}
			}
			if len(names) > 0 {
				m.dmDisplayNames[ch.ID] = strings.Join(names, ", ")
			}
		}
	}
}

func (m *Model) resolvePostUsers(posts []*model.Post) {
	for _, p := range posts {
		if _, ok := m.users[p.UserID]; !ok {
			m.users[p.UserID] = nil // placeholder
		}
	}
}

func (m Model) fetchMissingUsers() tea.Cmd {
	var missing []string
	for id, u := range m.users {
		if u == nil {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return FetchUsersByIDs(m.client, missing)
}

func (m Model) usernameMap() map[string]string {
	names := make(map[string]string)
	for id, u := range m.users {
		if u != nil {
			names[id] = u.Username
		}
	}
	return names
}

func (m Model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Ctrl+C quits from login screen too.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, m.keys.Quit) {
		return m, tea.Quit
	}

	// Handle login success from async command.
	if successMsg, ok := msg.(login.LoginSuccessMsg); ok {
		return m.handleLoginSuccess(successMsg)
	}

	var cmd tea.Cmd
	m.loginModel, cmd = m.loginModel.Update(msg)
	return m, cmd
}

func (m Model) handleLoginSuccess(msg login.LoginSuccessMsg) (tea.Model, tea.Cmd) {
	if m.tokenStore != nil {
		m.tokenStore.Set(msg.Token)
	}
	if m.wsClient != nil {
		m.wsClient.SetToken(msg.Token)
	}

	// Persist session to disk.
	serverURL := ""
	if m.cfg != nil {
		serverURL = m.cfg.ServerURL
	}
	_ = m.sessionStore.Save(auth.StoredSession{
		ServerURL: serverURL,
		Token:     msg.Token,
		ExpiresAt: msg.ExpiresAt,
	})

	m.appState = AppStateRunning
	return m, m.initRunning()
}

func (m Model) handleAuthExpired() (tea.Model, tea.Cmd) {
	m.appState = AppStateReLogin
	m.wsConnected = false
	m.loginModel.Reset()
	_ = m.sessionStore.Clear()

	// Close existing WS connection.
	if m.wsClient != nil {
		_ = m.wsClient.Close()
	}

	return m, nil
}

func (m Model) handleLogout() (tea.Model, tea.Cmd) {
	if m.tokenStore != nil {
		m.tokenStore.Set("")
	}
	_ = m.sessionStore.Clear()

	m.appState = AppStateLogin
	m.wsConnected = false
	m.loginModel.Reset()

	if m.wsClient != nil {
		_ = m.wsClient.Close()
	}

	return m, nil
}

// activeChannelDisplayName resolves the human-readable name of the active
// channel: the other participant for DMs, the member list for group channels,
// and the display name otherwise.
func (m Model) activeChannelDisplayName() string {
	if m.activeChan == nil {
		return ""
	}
	switch m.activeChan.Type {
	case model.ChannelDirect:
		if m.me != nil {
			parts := strings.Split(m.activeChan.Name, "__")
			if len(parts) == 2 {
				otherID := parts[0]
				if otherID == m.me.ID {
					otherID = parts[1]
				}
				if u, ok := m.users[otherID]; ok && u != nil {
					return u.Username
				}
			}
		}
		return m.activeChan.DisplayName
	case model.ChannelGroup:
		if name := m.dmDisplayNames[m.activeChan.ID]; name != "" {
			return name
		}
		return m.activeChan.DisplayName
	default:
		return m.activeChan.DisplayName
	}
}

// placeOverlay places a foreground string on top of a background string at x, y.
// It is ANSI-aware: it uses charmbracelet/x/ansi to truncate styled lines
// without breaking escape sequences.
func placeOverlay(x, y int, fg, bg string) string {
	bgLines := splitLines(bg)
	fgLines := splitLines(fg)

	for i, fgLine := range fgLines {
		bgIdx := y + i
		if bgIdx < 0 || bgIdx >= len(bgLines) {
			continue
		}
		bgLine := bgLines[bgIdx]
		fgWidth := ansi.StringWidth(fgLine)
		bgWidth := ansi.StringWidth(bgLine)

		// Part of background before the overlay
		var before string
		if bgWidth >= x {
			before = ansi.Truncate(bgLine, x, "")
		} else {
			before = bgLine + strings.Repeat(" ", x-bgWidth)
		}

		// Part of background after the overlay
		afterStart := x + fgWidth
		var after string
		if bgWidth > afterStart {
			after = ansi.TruncateLeft(bgLine, afterStart, "")
		}

		bgLines[bgIdx] = before + fgLine + after
	}

	return strings.Join(bgLines, "\n")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
