package tui

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/infrashift/chit-tui/internal/api"
	"github.com/infrashift/chit-tui/internal/auth"
	"github.com/infrashift/chit-tui/internal/config"
	"github.com/infrashift/chit-tui/internal/model"
	"github.com/infrashift/chit-tui/internal/tui/chcreator"
	"github.com/infrashift/chit-tui/internal/tui/cmdpalette"
	"github.com/infrashift/chit-tui/internal/tui/dmpicker"
	"github.com/infrashift/chit-tui/internal/tui/input"
	"github.com/infrashift/chit-tui/internal/tui/login"
	"github.com/infrashift/chit-tui/internal/tui/mention"
	"github.com/infrashift/chit-tui/internal/tui/search"
	"github.com/infrashift/chit-tui/internal/tui/sidebar"
	"github.com/infrashift/chit-tui/internal/tui/skinpicker"
	"github.com/infrashift/chit-tui/internal/tui/tagpicker"
	"github.com/infrashift/chit-tui/internal/tui/thread"
	"github.com/infrashift/chit-tui/internal/tui/ui/styles"
	"github.com/infrashift/chit-tui/internal/tui/ui/theme"
	"github.com/infrashift/chit-tui/internal/tui/viewport"
	"github.com/infrashift/chit-tui/internal/ws"
)

// FocusArea defines which component has focus.
type FocusArea int

const (
	FocusSidebar FocusArea = iota
	FocusViewport
	FocusInput
	FocusThread
	FocusCmdPalette
	FocusSearch
	FocusDMPicker
	FocusSkinPicker
	FocusChCreator
	FocusTagPicker
)

const sidebarWidth = 30

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
	sidebar               sidebar.Model
	viewport              viewport.Model
	input                 input.Model
	thread                thread.Model
	cmdPalette            cmdpalette.Model
	search                search.Model
	mention               mention.Model
	dmPicker              dmpicker.Model
	skinPicker            skinpicker.Model
	chCreator             chcreator.Model
	tagPicker             tagpicker.Model
	loginModel            login.Model
	me                    *model.User
	activeTeam            *model.Team
	activeChan            *model.Channel
	channels              []*model.Channel
	dmChannels            []*model.Channel
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
		sidebar:        sidebar.New(s),
		viewport:       viewport.New(s),
		input:          input.New(s),
		thread:         thread.New(s),
		cmdPalette:     cmdpalette.New(s),
		search:         search.New(s),
		mention:        mention.New(s),
		dmPicker:       dmpicker.New(s),
		skinPicker:     skinpicker.New(s),
		chCreator:      chcreator.New(s),
		tagPicker:      tagpicker.New(s),
		loginModel:     login.New(s, kratosClient),
		users:          make(map[string]*model.User),
		channelMembers: make(map[string][]*model.ChannelMember),
		threadCounts:   make(map[string]int),
		postTags:       make(map[string][]*model.Tag),
		appState:       state,
		tokenStore:     tokenStore,
		kratosClient:   kratosClient,
		sessionStore:   sessionStore,
		focus:          FocusSidebar,
		keys:           DefaultKeyMap(),
		styles:         s,
	}
	m.sidebar.Focus()
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

	case AuthExpiredMsg:
		return m.handleAuthExpired()

	case LogoutMsg:
		return m.handleLogout()

	case tea.KeyMsg:
		// Global keybindings (Ctrl+C always quits)
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		// Intercept keys when skin picker is visible
		if m.skinPicker.Visible() {
			var cmd tea.Cmd
			m.skinPicker, cmd = m.skinPicker.Update(msg)
			return m, cmd
		}

		// Intercept keys when tag picker is visible
		if m.tagPicker.Visible() {
			var cmd tea.Cmd
			m.tagPicker, cmd = m.tagPicker.Update(msg)
			return m, cmd
		}

		// Intercept keys when channel creator is visible
		if m.chCreator.Visible() {
			var cmd tea.Cmd
			m.chCreator, cmd = m.chCreator.Update(msg)
			return m, cmd
		}

		// Intercept keys when DM picker is visible
		if m.dmPicker.Visible() {
			switch msg.Type {
			case tea.KeyUp, tea.KeyDown, tea.KeyEnter, tea.KeyEscape:
				var cmd tea.Cmd
				m.dmPicker, cmd = m.dmPicker.Update(msg)
				return m, cmd
			default:
				var cmd tea.Cmd
				m.dmPicker, cmd = m.dmPicker.Update(msg)
				return m, cmd
			}
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

		if key.Matches(msg, m.keys.NewDM) && m.focus != FocusDMPicker {
			cmd := m.setFocus(FocusDMPicker)
			m.dmPicker.Open()
			return m, cmd
		}

		if key.Matches(msg, m.keys.NewChannel) && m.focus != FocusChCreator && m.activeTeam != nil {
			cmd := m.setFocus(FocusChCreator)
			m.chCreator.Open(m.activeTeam.ID)
			return m, cmd
		}

		if key.Matches(msg, m.keys.CmdPalette) && m.focus != FocusCmdPalette {
			cmd := m.setFocus(FocusCmdPalette)
			m.cmdPalette.Open()
			return m, cmd
		}

		if key.Matches(msg, m.keys.Search) && m.focus != FocusSearch {
			cmd := m.setFocus(FocusSearch)
			m.search.Open()
			return m, cmd
		}

		if key.Matches(msg, m.keys.Escape) {
			if m.tagPicker.Visible() {
				m.tagPicker.Close()
				cmd := m.setFocus(FocusViewport)
				return m, cmd
			}
			if m.chCreator.Visible() {
				m.chCreator.Close()
				cmd := m.setFocus(FocusInput)
				return m, cmd
			}
			if m.dmPicker.Visible() {
				m.dmPicker.Close()
				if m.pendingPrivateChannel != nil {
					ch := m.pendingPrivateChannel
					m.pendingPrivateChannel = nil
					m.pendingMembers = nil
					cmd := m.setFocus(FocusInput)
					return m, tea.Batch(cmd, CreateChannel(m.client, ch))
				}
				cmd := m.setFocus(FocusInput)
				return m, cmd
			}
			if m.search.Visible() {
				m.search.Close()
				cmd := m.setFocus(FocusViewport)
				return m, cmd
			}
			if m.cmdPalette.Visible() {
				m.cmdPalette.Close()
				cmd := m.setFocus(FocusInput)
				return m, cmd
			}
			if m.thread.Visible() && m.focus == FocusThread {
				m.thread.SetVisible(false)
				cmd := m.setFocus(FocusViewport)
				m.resizeComponents()
				return m, cmd
			}
		}

		if key.Matches(msg, m.keys.ToggleThread) {
			m.thread.Toggle()
			var cmd tea.Cmd
			if m.thread.Visible() {
				cmd = m.setFocus(FocusThread)
			} else {
				cmd = m.setFocus(FocusViewport)
			}
			m.resizeComponents()
			return m, cmd
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
		m.sidebar.SetTeams(msg.Teams)
		if len(msg.Teams) > 0 {
			m.activeTeam = msg.Teams[0]
			cmds = append(cmds, FetchChannels(m.client, msg.Teams[0].ID))
		}
		return m, tea.Batch(cmds...)

	case ChannelsLoadedMsg:
		if msg.Err != nil {
			if api.IsUnauthorized(msg.Err) {
				return m.handleAuthExpired()
			}
			return m, m.setError(msg.Err)
		}
		initialLoad := m.channels == nil
		m.channels = msg.Channels
		m.sidebar.SetChannels(msg.Channels)
		if initialLoad && len(msg.Channels) > 0 {
			m.activeChan = msg.Channels[0]
			cmds = append(cmds, FetchPosts(m.client, msg.Channels[0].ID, 0, 60))
			cmds = append(cmds, ViewChannel(m.client, msg.Channels[0].ID))
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
		m.cmdPalette.SetCommands(msg.Commands)
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
			m.sidebar.SetUnread(msg.ChannelID, 0)
			m.sidebar.SetMention(msg.ChannelID, 0)
		}
		return m, nil

	case WSConnectedMsg:
		return m, ListenWebSocket(m.wsClient)

	case WebSocketEventMsg:
		return m.handleWSEvent(msg)

	case sidebar.ChannelSelectedMsg:
		if m.activeChan != nil && m.activeChan.ID != msg.Channel.ID {
			cmds = append(cmds, ViewChannel(m.client, m.activeChan.ID))
		}
		m.activeChan = msg.Channel
		cmds = append(cmds, FetchPosts(m.client, msg.Channel.ID, 0, 60))
		cmds = append(cmds, ViewChannel(m.client, msg.Channel.ID))
		m.thread.SetVisible(false)
		m.threadCounts = make(map[string]int)
		m.viewport.SetThreadCounts(m.threadCounts)
		m.resizeComponents()
		return m, tea.Batch(cmds...)

	case sidebar.BackToTeamsMsg:
		if m.activeChan != nil {
			cmds = append(cmds, ViewChannel(m.client, m.activeChan.ID))
		}
		m.activeChan = nil
		return m, tea.Batch(cmds...)

	case sidebar.TeamSelectedMsg:
		m.activeTeam = msg.Team
		cmds = append(cmds, FetchChannels(m.client, msg.Team.ID))
		return m, tea.Batch(cmds...)

	case viewport.PostSelectedMsg:
		cmds = append(cmds, FetchThread(m.client, msg.Post.ID))
		cmd := m.setFocus(FocusThread)
		cmds = append(cmds, cmd)
		m.resizeComponents()
		return m, tea.Batch(cmds...)

	case input.SendMsg:
		if m.activeChan != nil && m.me != nil {
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
		}
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
		cmd := m.setFocus(FocusCmdPalette)
		m.cmdPalette.Open()
		return m, cmd

	case thread.ReplyMsg:
		if m.activeChan != nil && m.me != nil {
			post := &model.Post{
				ChannelID: m.activeChan.ID,
				UserID:    m.me.ID,
				RootID:    msg.RootID,
				Content:   msg.Content,
			}
			cmds = append(cmds, CreatePost(m.client, post))
		}
		return m, tea.Batch(cmds...)

	case cmdpalette.CommandSelectedMsg:
		cmd := m.setFocus(FocusInput)
		return m, cmd

	case search.SubmitMsg:
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
			cmd := m.setError(msg.Err)
			m.search.SetError(msg.Err.Error())
			return m, cmd
		}
		if msg.Posts != nil {
			m.search.SetResults(msg.Posts.Order)
		}
		return m, nil

	case search.ResultSelectedMsg:
		m.search.Close()
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
		m.sidebar.SetDMChannels(msg.Channels)
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
		m.dmPicker.SetResults(filtered)
		return m, nil

	case dmpicker.UserPickedMsg:
		m.dmPicker.Close()
		if m.me != nil {
			cmds = append(cmds, CreateDMChannel(m.client, m.me.ID, msg.User.ID))
		}
		return m, tea.Batch(cmds...)

	case DMCreatedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if msg.Channel != nil {
			m.activeChan = msg.Channel
			m.sidebar.ActiveChanID = msg.Channel.ID
			// Add to DM list if not already present
			found := false
			for _, ch := range m.dmChannels {
				if ch.ID == msg.Channel.ID {
					found = true
					break
				}
			}
			if !found {
				m.dmChannels = append([]*model.Channel{msg.Channel}, m.dmChannels...)
				m.sidebar.SetDMChannels(m.dmChannels)
			}
			m.resolveDMDisplayNames()
			cmds = append(cmds, FetchPosts(m.client, msg.Channel.ID, 0, 60))
			cmds = append(cmds, ViewChannel(m.client, msg.Channel.ID))
			cmds = append(cmds, FetchDMChannels(m.client))
		}
		return m, tea.Batch(cmds...)

	case dmpicker.GroupPickedMsg:
		m.dmPicker.Close()
		if m.me != nil {
			ids := []string{m.me.ID}
			for _, u := range msg.Users {
				ids = append(ids, u.ID)
			}
			cmds = append(cmds, CreateGroupChannel(m.client, ids))
		}
		return m, tea.Batch(cmds...)

	case GroupCreatedMsg:
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if msg.Channel != nil {
			m.activeChan = msg.Channel
			m.sidebar.ActiveChanID = msg.Channel.ID
			found := false
			for _, ch := range m.dmChannels {
				if ch.ID == msg.Channel.ID {
					found = true
					break
				}
			}
			if !found {
				m.dmChannels = append([]*model.Channel{msg.Channel}, m.dmChannels...)
				m.sidebar.SetDMChannels(m.dmChannels)
			}
			m.resolveDMDisplayNames()
			cmds = append(cmds, FetchPosts(m.client, msg.Channel.ID, 0, 60))
			cmds = append(cmds, ViewChannel(m.client, msg.Channel.ID))
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
			m.activeChan = msg.Channel
			m.sidebar.ActiveChanID = msg.Channel.ID
			m.channels = append([]*model.Channel{msg.Channel}, m.channels...)
			m.sidebar.SetChannels(m.channels)
			cmds = append(cmds, FetchPosts(m.client, msg.Channel.ID, 0, 60))
			cmds = append(cmds, ViewChannel(m.client, msg.Channel.ID))
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
		m.sidebar.SetStyles(newStyles)
		m.viewport.SetStyles(newStyles)
		m.input.SetStyles(newStyles)
		m.thread.SetStyles(newStyles)
		m.cmdPalette.SetStyles(newStyles)
		m.search.SetStyles(newStyles)
		m.mention.SetStyles(newStyles)
		m.dmPicker.SetStyles(newStyles)
		m.skinPicker.SetStyles(newStyles)
		m.chCreator.SetStyles(newStyles)
		m.tagPicker.SetStyles(newStyles)
		cmd := m.setFocus(FocusInput)
		return m, cmd

	case ClearErrMsg:
		if msg.Seq == m.errSeq {
			m.err = nil
		}
		return m, nil

	case ErrMsg:
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

	sidebarView := m.sidebar.View()

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		m.input.View(),
	)

	var layout string
	if m.thread.Visible() {
		layout = lipgloss.JoinHorizontal(lipgloss.Top,
			sidebarView,
			mainContent,
			m.thread.View(),
		)
	} else {
		layout = lipgloss.JoinHorizontal(lipgloss.Top,
			sidebarView,
			mainContent,
		)
	}

	// Overlay command palette
	if m.cmdPalette.Visible() {
		paletteView := m.cmdPalette.View()
		paletteWidth := lipgloss.Width(paletteView)
		paletteX := (m.width - paletteWidth) / 2
		paletteY := 2
		layout = placeOverlay(paletteX, paletteY, paletteView, layout)
	}

	// Overlay mention autocomplete
	if m.mention.Visible() {
		mentionView := m.mention.View()
		mentionHeight := lipgloss.Height(mentionView)
		inputHeight := 5
		mentionX := sidebarWidth + 1
		mentionY := m.height - inputHeight - mentionHeight - 1
		if mentionY < 0 {
			mentionY = 0
		}
		layout = placeOverlay(mentionX, mentionY, mentionView, layout)
	}

	// Overlay DM picker
	if m.dmPicker.Visible() {
		pickerView := m.dmPicker.View()
		pickerWidth := lipgloss.Width(pickerView)
		pickerX := (m.width - pickerWidth) / 2
		pickerY := 2
		layout = placeOverlay(pickerX, pickerY, pickerView, layout)
	}

	// Overlay skin picker
	if m.skinPicker.Visible() {
		pickerView := m.skinPicker.View()
		pickerWidth := lipgloss.Width(pickerView)
		pickerX := (m.width - pickerWidth) / 2
		pickerY := 2
		layout = placeOverlay(pickerX, pickerY, pickerView, layout)
	}

	// Overlay channel creator
	if m.chCreator.Visible() {
		creatorView := m.chCreator.View()
		creatorWidth := lipgloss.Width(creatorView)
		creatorX := (m.width - creatorWidth) / 2
		creatorY := 2
		layout = placeOverlay(creatorX, creatorY, creatorView, layout)
	}

	// Overlay tag picker
	if m.tagPicker.Visible() {
		pickerView := m.tagPicker.View()
		pickerWidth := lipgloss.Width(pickerView)
		pickerX := (m.width - pickerWidth) / 2
		pickerY := 2
		layout = placeOverlay(pickerX, pickerY, pickerView, layout)
	}

	// Overlay search
	if m.search.Visible() {
		searchView := m.search.View()
		searchWidth := lipgloss.Width(searchView)
		searchX := (m.width - searchWidth) / 2
		searchY := 2
		layout = placeOverlay(searchX, searchY, searchView, layout)
	}

	// Status bar
	status := m.statusBar()
	return lipgloss.JoinVertical(lipgloss.Left, layout, status)
}

func (m *Model) setFocus(area FocusArea) tea.Cmd {
	m.sidebar.Blur()
	m.viewport.Blur()
	m.input.Blur()
	m.thread.Blur()
	m.cmdPalette.Blur()
	m.search.Blur()
	m.dmPicker.Blur()
	m.skinPicker.Blur()
	m.chCreator.Blur()
	m.tagPicker.Blur()

	m.focus = area
	switch area {
	case FocusSidebar:
		m.sidebar.Focus()
	case FocusViewport:
		m.viewport.Focus()
	case FocusInput:
		return m.input.Focus()
	case FocusThread:
		return m.thread.Focus()
	case FocusCmdPalette:
		m.cmdPalette.Focus()
	case FocusSearch:
		m.search.Focus()
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

func (m *Model) cycleFocus(dir int) tea.Cmd {
	areas := []FocusArea{FocusSidebar, FocusViewport, FocusInput}
	if m.thread.Visible() {
		areas = append(areas, FocusThread)
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
	threadWidth := 0
	if m.thread.Visible() {
		threadWidth = m.width / 4
	}

	mainWidth := m.width - sidebarWidth - threadWidth
	inputHeight := 5
	vpHeight := m.height - inputHeight - 1 // -1 for status bar

	m.sidebar.SetSize(sidebarWidth, m.height-1)
	m.viewport.SetSize(mainWidth, vpHeight)
	m.input.SetSize(mainWidth, inputHeight)
	m.thread.SetSize(threadWidth, m.height-1)
	m.cmdPalette.SetSize(m.width, m.height)
	m.search.SetSize(m.width, m.height)
	m.dmPicker.SetSize(m.width, m.height)
	m.skinPicker.SetSize(m.width, m.height)
	m.chCreator.SetSize(m.width, m.height)
	m.tagPicker.SetSize(m.width, m.height)
}

func (m Model) delegateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focus {
	case FocusSidebar:
		m.sidebar, cmd = m.sidebar.Update(msg)
	case FocusViewport:
		m.viewport, cmd = m.viewport.Update(msg)
	case FocusInput:
		m.input, cmd = m.input.Update(msg)
	case FocusThread:
		m.thread, cmd = m.thread.Update(msg)
	case FocusCmdPalette:
		m.cmdPalette, cmd = m.cmdPalette.Update(msg)
	case FocusSearch:
		m.search, cmd = m.search.Update(msg)
	case FocusDMPicker:
		m.dmPicker, cmd = m.dmPicker.Update(msg)
	case FocusSkinPicker:
		m.skinPicker, cmd = m.skinPicker.Update(msg)
	case FocusChCreator:
		m.chCreator, cmd = m.chCreator.Update(msg)
	case FocusTagPicker:
		m.tagPicker, cmd = m.tagPicker.Update(msg)
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
				if m.thread.Visible() && m.thread.RootPost() != nil && m.thread.RootPost().ID == p.RootID {
					m.thread.AppendReply(&p)
				}
			}
			m.resolvePostUsers([]*model.Post{&p})
			if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
				cmds = append(cmds, fetchCmd)
			}
		} else {
			cur := m.sidebar.UnreadCounts[p.ChannelID]
			m.sidebar.SetUnread(p.ChannelID, cur+1)
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
					if p.RootID != "" && m.thread.Visible() && m.thread.RootPost() != nil && m.thread.RootPost().ID == p.RootID {
						m.thread.AppendReply(&p)
					}
				}
			}
		}

	case model.WebSocketEventMentioned:
		if evt.Broadcast != nil && evt.Broadcast.ChannelID != "" {
			chanID := evt.Broadcast.ChannelID
			cur := m.sidebar.MentionCounts[chanID]
			m.sidebar.SetMention(chanID, cur+1)
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
			m.sidebar.SetUnread(channelID, unread)
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
			m.sidebar.SetMention(channelID, mem.MentionCount)
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
				m.sidebar.SetDMDisplayName(ch.ID, name)
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
				m.sidebar.SetDMDisplayName(ch.ID, strings.Join(names, ", "))
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
	m.loginModel.Reset()

	if m.wsClient != nil {
		_ = m.wsClient.Close()
	}

	return m, nil
}

func (m Model) statusBar() string {
	team := ""
	if m.activeTeam != nil {
		team = m.activeTeam.DisplayName
	}
	ch := ""
	if m.activeChan != nil {
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
						ch = u.Username
					} else {
						ch = m.activeChan.DisplayName
					}
				} else {
					ch = m.activeChan.DisplayName
				}
			} else {
				ch = m.activeChan.DisplayName
			}
		case model.ChannelGroup:
			if name := m.sidebar.GetDMDisplayName(m.activeChan.ID); name != "" {
				ch = name
			} else {
				ch = m.activeChan.DisplayName
			}
		default:
			ch = m.activeChan.DisplayName
		}
	}
	user := ""
	if m.me != nil {
		user = m.me.Username
	}

	errStr := ""
	if m.err != nil {
		errStr = m.styles.ErrorText.Render(" " + m.err.Error())
	}

	left := m.styles.StatusBar.Render(team + " > " + ch)
	right := m.styles.StatusBar.Render(user)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - lipgloss.Width(errStr)
	if gap < 0 {
		gap = 0
	}

	gapStyle := m.styles.StatusBar.Padding(0, 0)
	return left + errStr + gapStyle.Render(strings.Repeat(" ", gap)) + right
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
