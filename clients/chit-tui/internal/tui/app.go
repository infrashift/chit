package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

// inputHeight is the height of the message input box, in rows. Layout and
// mouse hit-testing both depend on it, so it lives in one place.
const inputHeight = 5

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
	// History paging. historyPage is the newest page already loaded;
	// loadingOlder guards against firing repeatedly while a fetch is in
	// flight, and historyExhausted stops asking once the server runs out.
	historyPage      int
	loadingOlder     bool
	historyExhausted bool
	// editingPostID is set while a post is being edited; sending replaces
	// that post instead of creating a new one.
	editingPostID string
	// searchWasGlobal records whether the last search spanned every channel,
	// so choosing a result knows it may have to switch channel first.
	searchWasGlobal bool
	// searchTerm is the last submitted search, kept so a chosen result can
	// be highlighted in the history.
	searchTerm string
}

// Connection-state notices. They travel through the same transient status
// line as errors because that is the only place the UI can say something
// in passing.
var (
	errDisconnected = errors.New("connection lost — reconnecting")
	errReconnected  = errors.New("reconnected — reloading messages")
)

// errSearchHitNotLoaded reports a result that is outside the loaded history.
var errSearchHitNotLoaded = errors.New(
	"that message is older than the loaded history; scroll back to reach it")

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

		// Copy the mouse selection, or the post under the cursor when there
		// is none — so the key is useful without a mouse.
		if key.Matches(msg, m.keys.Copy) && m.focus == FocusViewport {
			text := m.viewport.SelectedText()
			if text == "" {
				if p := m.viewport.SelectedPost(); p != nil {
					text = p.Content
				}
			}
			if text != "" {
				return m, copyToClipboard(text)
			}
		}

		// Editing loads the post back into the input; sending replaces it.
		// Only your own posts, matching what the server enforces.
		if key.Matches(msg, m.keys.Edit) && m.focus == FocusViewport {
			if p := m.ownSelectedPost(); p != nil {
				m.editingPostID = p.ID
				m.input.SetValue(p.Content)
				return m, m.setFocus(FocusInput)
			}
		}

		if key.Matches(msg, m.keys.Delete) && m.focus == FocusViewport {
			if p := m.ownSelectedPost(); p != nil {
				return m, DeletePost(m.client, p.ID)
			}
		}

		// Pinning is a channel-level act, so unlike edit and delete it works
		// on anyone's post.
		if key.Matches(msg, m.keys.Pin) && m.focus == FocusViewport {
			if p := m.viewport.SelectedPost(); p != nil {
				return m, SetPostPinned(m.client, p.ID, !p.IsPinned)
			}
		}

		if key.Matches(msg, m.keys.TagPicker) {
			// In the history pane the subject is the post under the cursor;
			// in a thread there is no cursor, so it is the root post.
			var sel *model.Post
			switch m.focus {
			case FocusViewport:
				sel = m.viewport.SelectedPost()
			case FocusThread:
				sel = m.thread.RootPost()
			}
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
				m.channelAutoSelected = true
				m.activeChan = msg.Channels[0]
				cmds = append(cmds, FetchPosts(m.client, msg.Channels[0].ID, 0, 60))
				cmds = append(cmds, ViewChannel(m.client, msg.Channels[0].ID))
			}
		}
		return m, tea.Batch(cmds...)

	case PostsLoadedMsg:
		m.viewport.SetLoading(false)
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		m.viewport.SetLoading(false)
		m.viewport.SetPosts(msg.Posts.Order)
		m.historyPage = 0
		m.loadingOlder = false
		// A short first page means there is nothing older to ask for.
		m.historyExhausted = len(msg.Posts.Order) < historyPageSize
		m.resolvePostUsers(msg.Posts.Order)
		m.postTags = make(map[string][]*model.Tag)
		if ids := postIDs(msg.Posts.Order); len(ids) > 0 {
			cmds = append(cmds, FetchPostsTags(m.client, ids))
		}
		if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
			cmds = append(cmds, fetchCmd)
		}
		return m, tea.Batch(cmds...)

	case PostCreatedMsg:
		if msg.Err != nil {
			cmd := m.setError(msg.Err)
			m.pendingPostTags = nil
			return m, cmd
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
		// Keep listening for the next transition before doing anything else,
		// or a single drop would be the last one ever reported.
		cmds = append(cmds, ListenWSState(m.wsClient))

		if msg.Unauthorized {
			m.wsConnected = false
			return m.handleAuthExpired()
		}

		wasConnected := m.wsConnected
		m.wsConnected = msg.Connected

		if msg.Connected && !wasConnected {
			// Events that arrived while the socket was down are gone for
			// good — the stream has no replay — so re-read the channel
			// rather than leaving a silent hole in the history.
			if m.activeChan != nil {
				cmds = append(cmds, FetchPosts(m.client, m.activeChan.ID, 0, 60))
			}
			cmds = append(cmds, m.setError(errReconnected))
		}
		if !msg.Connected && wasConnected {
			cmds = append(cmds, m.setError(errDisconnected))
		}
		return m, tea.Batch(cmds...)

	case WSConnectedMsg:
		m.wsConnected = true
		// Both listeners start here: one for events, one for transport
		// state. Returning only the first is what left disconnects silent.
		return m, tea.Batch(ListenWebSocket(m.wsClient), ListenWSState(m.wsClient))

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
		if m.editingPostID != "" {
			id := m.editingPostID
			m.editingPostID = ""
			m.pendingPostTags = nil
			return m, EditPost(m.client, id, content)
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

		// /skin and /logout act on this client alone; the server knows
		// nothing about them.
		switch trimmed {
		case "/skin", "/theme":
			m.skinPicker.SetSkins(theme.ListAvailable())
			cmd := m.setFocus(FocusSkinPicker)
			m.skinPicker.Open()
			return m, cmd
		case "/logout":
			return m.handleLogout()
		case "/leave":
			// Handled here rather than server-side: there is no leave command
			// in the registry, and the REST endpoint already permits a member
			// to remove themselves.
			if m.activeChan == nil || m.me == nil {
				return m, nil
			}
			return m, LeaveChannel(m.client, m.activeChan.ID, m.me.ID)
		case "/":
			// A bare slash is a request to browse, not to send.
			return m, m.openPalette("/")
		}

		// Everything else goes to the server, which owns the command
		// registry, authorization, and the "unknown command" reply. Anything
		// that is not a command — a path like /usr/local/bin — is persisted
		// as an ordinary message.
		if m.activeChan == nil || m.me == nil {
			return m, nil
		}
		return m, CreatePost(m.client, &model.Post{
			ChannelID: m.activeChan.ID,
			UserID:    m.me.ID,
			Content:   trimmed,
		})

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
			for _, t := range m.allTags {
				if strings.EqualFold(t.Name, name) {
					tagIDs = append(tagIDs, t.ID)
					break
				}
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
		if err := config.SaveTheme(msg.Name); err != nil {
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
		m.loadingOlder = false
		if msg.Err != nil {
			return m, m.setError(msg.Err)
		}
		if m.activeChan == nil || msg.ChannelID != m.activeChan.ID {
			// The reader moved on while this was in flight.
			return m, nil
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
		m.resolvePostUsers(older)
		if ids := postIDs(older); len(ids) > 0 {
			cmds = append(cmds, FetchPostsTags(m.client, ids))
		}
		if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
			cmds = append(cmds, fetchCmd)
		}
		return m, tea.Batch(cmds...)

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
		return m, nil

	case PostsTagsLoadedMsg:
		if msg.Err != nil {
			// Tags are decoration; a failure here should not disturb the
			// channel, but it should not vanish silently either.
			return m, m.setError(msg.Err)
		}
		for postID, tags := range msg.Tags {
			m.postTags[postID] = tags
			names := make([]string, 0, len(tags))
			for _, t := range tags {
				names = append(names, t.Name)
			}
			m.viewport.SetPostTags(postID, names)
		}
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

	// Replying needs a focused history pane with a post under the cursor;
	// the button appears only then, which is also the hint that the pane has
	// to be focused first.
	m.actionBar.SetCanReply(m.mainPane == paneChannel &&
		m.focus == FocusViewport && m.viewport.SelectedPost() != nil)
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
	// A highlight from a search in the previous channel would otherwise
	// carry over and mark unrelated text here.
	m.clearSearchHighlight()
	// DM/group channels have no team; keep the last active team then.
	if ch.TeamID != "" {
		if t := m.teamByID(ch.TeamID); t != nil {
			m.activeTeam = t
		}
	}
	m.viewport.SetPosts(nil)
	m.viewport.SetLoading(true)
	cmds = append(cmds, FetchPosts(m.client, ch.ID, 0, historyPageSize))
	cmds = append(cmds, ViewChannel(m.client, ch.ID))
	// Members are only read for the active channel, to build the @-mention
	// list, so they are fetched on entry rather than for every channel in
	// every team up front.
	if _, have := m.channelMembers[ch.ID]; !have {
		cmds = append(cmds, FetchChannelMembers(m.client, ch.ID))
	}
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
// clearSearchHighlight removes match highlighting. A highlight that outlives
// the search reads as if the term were still active.
func (m *Model) clearSearchHighlight() {
	m.searchTerm = ""
	m.viewport.SetSearchTerm("")
}

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
		// Scrolling or moving the cursor may have reached the oldest loaded
		// post, which is the cue to fetch the page before it.
		if older := m.maybeLoadOlder(); older != nil {
			return m, tea.Batch(cmd, older)
		}
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
		p := decodePost(evt.Data)
		if p == nil {
			// A payload the client cannot read means the two sides disagree
			// about the schema. Dropped silently, that looks like messages
			// simply never arriving.
			slog.Warn("could not decode a posted event", "event", evt.Event)
			return m, tea.Batch(cmds...)
		}
		if m.activeChan != nil && p.ChannelID == m.activeChan.ID {
			// The sender already appended this from the HTTP response.
			if m.viewport.HasPost(p.ID) {
				return m, tea.Batch(cmds...)
			}
			m.viewport.AppendPost(p)
			if p.RootID != "" {
				m.threadCounts[p.RootID]++
				m.viewport.SetThreadCounts(m.threadCounts)
				if m.mainPane == paneThread && m.thread.RootPost() != nil && m.thread.RootPost().ID == p.RootID {
					m.thread.AppendReply(p)
				}
			}
			m.resolvePostUsers([]*model.Post{p})
			if fetchCmd := m.fetchMissingUsers(); fetchCmd != nil {
				cmds = append(cmds, fetchCmd)
			}
		} else {
			m.setUnread(p.ChannelID, m.unread[p.ChannelID]+1)
		}

	case model.WebSocketEventPostEdited:
		// An edit carries the whole post, so it replaces what is displayed.
		if p := decodePost(evt.Data); p != nil {
			m.viewport.UpdatePost(p)
			if m.mainPane == paneThread {
				m.thread.UpdatePost(p)
			}
		} else {
			slog.Warn("could not decode a post_edited event")
		}

	case model.WebSocketEventPostPinned, model.WebSocketEventPostUnpinned:
		// Unlike an edit, these carry only an ID, so the flag is flipped on
		// the post already held rather than replacing it.
		if postID, _ := evt.Data["post_id"].(string); postID != "" {
			m.viewport.SetPinned(postID, evt.Event == model.WebSocketEventPostPinned)
		}

	case model.WebSocketEventPostDeleted:
		postID, _ := evt.Data["post_id"].(string)
		if postID != "" {
			m.viewport.RemovePost(postID)
			// Leave a thread whose root just disappeared, rather than
			// showing an empty pane.
			if m.mainPane == paneThread && m.thread.RootPost() != nil &&
				m.thread.RootPost().ID == postID {
				cmds = append(cmds, m.closeThread())
			}
		}

	case model.WebSocketEventChannelUpdated:
		if ch := decodeChannel(evt.Data); ch != nil {
			m.replaceChannel(ch)
		}

	case model.WebSocketEventChannelDeleted:
		channelID, _ := evt.Data["channel_id"].(string)
		if channelID != "" {
			cmds = append(cmds, m.removeChannel(channelID)...)
		}

	case model.WebSocketEventUserRemoved:
		channelID, _ := evt.Data["channel_id"].(string)
		userID, _ := evt.Data["user_id"].(string)
		if m.me != nil && userID == m.me.ID {
			// Removed from a channel: it is no longer reachable.
			cmds = append(cmds, m.removeChannel(channelID)...)
		} else if channelID != "" {
			// Someone else left; the @-mention list is now stale.
			delete(m.channelMembers, channelID)
			if m.activeChan != nil && m.activeChan.ID == channelID {
				cmds = append(cmds, FetchChannelMembers(m.client, channelID))
			}
		}

	case model.WebSocketEventCommandResponse:
		// Ephemeral: broadcast to the invoking user only, never persisted.
		// It is shown as a post so multi-line output such as /help stays
		// readable, and it disappears on the next channel load.
		text, _ := evt.Data["text"].(string)
		channelID, _ := evt.Data["channel_id"].(string)
		slug, _ := evt.Data["command_slug"].(string)

		if text != "" && m.activeChan != nil && channelID == m.activeChan.ID {
			m.viewport.AppendPost(&model.Post{
				ID:        commandResponseUserID + ":" + slug,
				ChannelID: channelID,
				UserID:    commandResponseUserID,
				Content:   text,
				CreateAt:  time.Now().UnixMilli(),
			})
			m.registerCommandResponseAuthor(slug)
		}

	case model.WebSocketEventThreadUpdated:
		if threadData, ok := evt.Data["thread"]; ok {
			var t model.Thread
			if err := reDecode(threadData, &t); err != nil {
				slog.Warn("could not decode a thread_updated payload", "error", err)
			} else {
				m.threadCounts[t.PostID] = t.ReplyCount
				m.viewport.SetThreadCounts(m.threadCounts)
			}
		}
		if postData, ok := evt.Data["post"]; ok {
			var p model.Post
			if err := reDecode(postData, &p); err != nil {
				slog.Warn("could not decode a thread_updated post", "error", err)
			} else if p.RootID != "" && m.mainPane == paneThread &&
				m.thread.RootPost() != nil && m.thread.RootPost().ID == p.RootID {
				m.thread.AppendReply(&p)
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
	// A failed save means this login will not survive a restart. It does not
	// stop the session working now, so it is a warning rather than a failure.
	var saveCmd tea.Cmd
	if err := m.sessionStore.Save(auth.StoredSession{
		ServerURL: serverURL,
		Token:     msg.Token,
		ExpiresAt: msg.ExpiresAt,
	}); err != nil {
		saveCmd = m.setError(fmt.Errorf("signed in, but the session could not be saved: %w", err))
	}

	m.appState = AppStateRunning
	return m, tea.Batch(m.initRunning(), saveCmd)
}

func (m Model) handleAuthExpired() (tea.Model, tea.Cmd) {
	m.appState = AppStateReLogin
	m.wsConnected = false
	m.loginModel.Reset()
	// A stored token that cannot be cleared would be retried on next start
	// and fail the same way, so this is worth knowing about.
	if err := m.sessionStore.Clear(); err != nil {
		slog.Warn("could not clear the stored session", "error", err)
	}

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
	// Logging out and leaving the token on disk would sign the user straight
	// back in on next start, which is the opposite of what they asked for.
	var clearCmd tea.Cmd
	if err := m.sessionStore.Clear(); err != nil {
		clearCmd = m.setError(fmt.Errorf("signed out, but the stored session remains: %w", err))
	}

	m.appState = AppStateLogin
	m.wsConnected = false
	m.loginModel.Reset()

	if m.wsClient != nil {
		_ = m.wsClient.Close()
	}

	return m, clearCmd
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

// commandResponseUserID labels ephemeral command output. It is not a real
// user, so it can never collide with one: user IDs are UUIDs.
const commandResponseUserID = "chit:command-response"

// registerCommandResponseAuthor names the pseudo-author after the command that
// produced the output, so a reply reads as coming from "/help" rather than
// from whoever happened to type it.
func (m *Model) registerCommandResponseAuthor(slug string) {
	name := "/" + slug
	if slug == "" {
		name = "command"
	}
	names := make(map[string]string, len(m.users)+1)
	for id, u := range m.users {
		names[id] = u.Username
	}
	names[commandResponseUserID] = name
	m.viewport.SetUsernames(names)
}

// errorCarrier is implemented by every message that reports a failed request.
type errorCarrier interface{ requestError() error }

// msgError extracts the error a message carries, if any.
func msgError(msg tea.Msg) error {
	if c, ok := msg.(errorCarrier); ok {
		return c.requestError()
	}
	return nil
}

// reDecode round-trips a decoded event field back through JSON into a typed
// value. Event payloads arrive as generic maps, so this is how they are read
// without asserting field by field.
func reDecode(v any, out any) error {
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
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	var p model.Post
	if err := json.Unmarshal(raw, &p); err != nil || p.ID == "" {
		return nil
	}
	return &p
}

// decodeChannel re-decodes an event payload into a channel.
func decodeChannel(data map[string]any) *model.Channel {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	var c model.Channel
	if err := json.Unmarshal(raw, &c); err != nil || c.ID == "" {
		return nil
	}
	return &c
}

// replaceChannel swaps an updated channel into every list holding it, so a
// rename shows up without a reload.
func (m *Model) replaceChannel(ch *model.Channel) {
	replace := func(list []*model.Channel) {
		for i, existing := range list {
			if existing.ID == ch.ID {
				list[i] = ch
			}
		}
	}
	replace(m.channels)
	replace(m.dmChannels)
	for _, list := range m.channelsByTeam {
		replace(list)
	}
	if m.activeChan != nil && m.activeChan.ID == ch.ID {
		m.activeChan = ch
	}
	m.palette.SetChannels(m.channels)
	m.palette.SetDMChannels(m.dmChannels)
}

// removeChannel drops a channel that no longer exists or is no longer
// reachable, moving off it first if it is the one being viewed.
func (m *Model) removeChannel(channelID string) []tea.Cmd {
	if channelID == "" {
		return nil
	}

	drop := func(list []*model.Channel) []*model.Channel {
		out := list[:0]
		for _, ch := range list {
			if ch.ID != channelID {
				out = append(out, ch)
			}
		}
		return out
	}
	m.channels = drop(m.channels)
	m.dmChannels = drop(m.dmChannels)
	for team, list := range m.channelsByTeam {
		m.channelsByTeam[team] = drop(list)
	}
	delete(m.channelMembers, channelID)
	delete(m.unread, channelID)
	delete(m.mentions, channelID)

	m.palette.SetChannels(m.channels)
	m.palette.SetDMChannels(m.dmChannels)

	var cmds []tea.Cmd
	if m.activeChan != nil && m.activeChan.ID == channelID {
		m.activeChan = nil
		m.viewport.SetPosts(nil)
		if len(m.channels) > 0 {
			cmds = append(cmds, m.selectChannel(m.channels[0]))
		}
		cmds = append(cmds, m.setError(errChannelGone))
	}
	return cmds
}

// errSearchHitElsewhere explains why the view changed channel.
var errSearchHitElsewhere = errors.New("opened the channel containing that message")

// errLoadingOlder is a notice, not a failure: fetching a page of older
// history can take a moment and the view does not otherwise change.
var errLoadingOlder = errors.New("loading older messages…")

// errChannelGone explains why the view moved on its own.
var errChannelGone = errors.New("this channel is no longer available")

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
	if m.activeChan == nil || m.loadingOlder || m.historyExhausted {
		return nil
	}
	if !m.viewport.AtTop() {
		return nil
	}
	m.loadingOlder = true
	return tea.Batch(
		FetchOlderPosts(m.client, m.activeChan.ID, m.historyPage+1, historyPageSize),
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

// SelectedPostID returns the ID of the post under the history cursor, or "".
// Exported for tests, which cannot reach the viewport's cursor otherwise.
func (m Model) SelectedPostID() string {
	if p := m.viewport.SelectedPost(); p != nil {
		return p.ID
	}
	return ""
}

// HasSelection reports whether the history pane holds a selection. Exported
// for tests, which cannot reach the viewport otherwise.
func (m Model) HasSelection() bool { return m.viewport.HasSelection() }

// channelByID finds a channel across the team lists and DMs. A search that
// spans every channel can return a hit from any of them.
func (m Model) channelByID(id string) *model.Channel {
	for _, ch := range m.channels {
		if ch.ID == id {
			return ch
		}
	}
	for _, ch := range m.dmChannels {
		if ch.ID == id {
			return ch
		}
	}
	for _, list := range m.channelsByTeam {
		for _, ch := range list {
			if ch.ID == id {
				return ch
			}
		}
	}
	return nil
}
