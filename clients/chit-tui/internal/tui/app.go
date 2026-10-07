package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
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
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/threadinbox"
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
	FocusThreadInbox
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
	cfg      *config.Config
	client   api.ChitClient
	wsClient ws.WSClient
	// wsListening is set once the event and state listeners are running.
	wsListening           bool
	viewport              viewport.Model
	input                 input.Model
	thread                thread.Model
	actionBar             actionbar.Model
	palette               palette.Model
	help                  help.Model
	mention               mention.Model
	dmPicker              dmpicker.Model
	skinPicker            skinpicker.Model
	threadInbox           threadinbox.Model
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
	pendingPrivateChannel *model.Channel
	pendingMembers        []string
	// pendingGroupChannel marks the member picker as serving /group rather
	// than private-channel creation. The two share the picker but not what
	// happens to the result.
	pendingGroupChannel bool
	appState            AppState
	tokenStore          *auth.TokenStore
	kratosClient        *auth.KratosClient
	sessionStore        *auth.SessionStore
	focus               FocusArea
	mainPane            mainPane
	channelAutoSelected bool
	wsConnected         bool
	lastWSSeq           int64
	keys                KeyMap
	styles              styles.Styles
	width               int
	height              int
	err                 error
	errSeq              uint64
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
	// usersRequested marks user IDs already looked up.
	usersRequested map[string]bool
	// commandAuthors names the pseudo-author of each command's output, by
	// user ID; commandResponses numbers the outputs so each has its own ID.
	commandAuthors   map[string]string
	commandResponses int
	// confirmDeleteID is the post awaiting a second delete keypress.
	confirmDeleteID string
	// threadRootID is the root of the thread in the main pane, set as soon
	// as it opens, before the thread itself has loaded.
	threadRootID string
	// userQuery is the latest user search sent; older answers are dropped.
	userQuery string
	// expiredUserID is who was signed in when the session expired. Their
	// state is kept for the re-login, which may not be theirs.
	expiredUserID string
}

// Connection-state notices. They travel through the same transient status
// line as errors because that is the only place the UI can say something
// in passing.
var (
	errDisconnected = errors.New("connection lost — reconnecting")
	errReconnected  = errors.New("reconnected — reloading messages")
	errDesynced     = errors.New("fell behind the server — reloading messages")
	// Not a failure, but the status line is the only place to ask.
	errConfirmDelete = errors.New("delete this message? Press d again to confirm, any other key to cancel")
)

// threadInboxPageSize bounds the inbox at one screenful's worth. Following
// more threads than this is possible; paging through them is not yet.
const threadInboxPageSize = 50

// minGroupChannelMembers matches the server's lower bound. Below it the
// conversation is a DM, which has its own flow.
const minGroupChannelMembers = 3

var errGroupTooSmall = errors.New(
	"a group needs at least three people — pick two others, or start a DM with @")

var (
	errNickUsage      = errors.New("usage: /nick <display name>")
	errUsernameUsage  = errors.New("usage: /username <handle>")
	errUsernameSpaces = errors.New("a username cannot contain spaces — try /nick for a display name")
	// Not a failure: setError is the only status-bar channel there is.
	errProfileSaved = errors.New("profile updated")
	errNoTeam       = errors.New("no team is active yet")
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
		threadInbox:    threadinbox.New(s),
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
		commandAuthors: make(map[string]string),
		usersRequested: make(map[string]bool),
		appState:       state,
		tokenStore:     tokenStore,
		kratosClient:   kratosClient,
		sessionStore:   sessionStore,
		focus:          FocusInput,
		keys:           DefaultKeyMap(),
		styles:         s,
	}
	// Seed the client-local commands rather than waiting for the server
	// registry, which never contains them and may never arrive at all.
	m.palette.SetCommands(clientCommands())

	// The palette shares the root's badge and DM-name maps by reference, so
	// root-side updates are visible without further plumbing.
	m.palette.SetCounts(m.unread, m.mentions)
	m.palette.SetDMDisplayNames(m.dmDisplayNames)
	_ = m.input.Focus()
	return m
}

// clientCommands are handled entirely by this client and never reach the
// server, so the server's registry does not list them. Typing one directly has
// always worked; browsing for one had not, because a bare "/" opens the
// palette on the server's list alone and these were absent from it.
func clientCommands() []*model.Command {
	return []*model.Command{
		{Slug: "theme", Description: "choose a theme (alias: /skin)"},
		{Slug: "group", Description: "start a group conversation with three or more people"},
		{Slug: "nick", Description: "change your display name"},
		{Slug: "username", Description: "change your username (breaks existing @mentions)"},
		{Slug: "threads", Description: "threads you follow in this team"},
		{Slug: "leave", Description: "leave the current channel"},
		{Slug: "logout", Description: "sign out and clear the stored session"},
	}
}

// mergeCommands appends the server's commands to the client's, dropping any
// the client already handles: a server entry of the same name would be routed
// to a handler that never runs.
func mergeCommands(local, remote []*model.Command) []*model.Command {
	seen := make(map[string]bool, len(local))
	for _, c := range local {
		seen[c.Slug] = true
	}
	merged := append([]*model.Command(nil), local...)
	for _, c := range remote {
		if c != nil && !seen[c.Slug] {
			merged = append(merged, c)
		}
	}
	return merged
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
			return WSConnectedMsg{Err: m.wsClient.Connect()}
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

		// A pending delete is confirmed by pressing the key again on the
		// same post, while the question is still on screen; once it has
		// timed out or been replaced, nothing is being asked. Anything else
		// cancels it and then does what it does.
		if id := m.confirmDeleteID; id != "" {
			m.confirmDeleteID = ""
			asking := errors.Is(m.err, errConfirmDelete)
			if asking {
				m.err = nil
			}
			if asking && key.Matches(msg, m.keys.Delete) && m.focus == FocusViewport {
				if p := m.ownSelectedPost(); p != nil && p.ID == id {
					return m, DeletePost(m.client, p.ID)
				}
			}
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
				m.cancelEdit()
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

		// Deleting cannot be undone and "d" is one stray keystroke away, so
		// it asks first.
		if key.Matches(msg, m.keys.Delete) && m.focus == FocusViewport {
			if p := m.ownSelectedPost(); p != nil {
				m.confirmDeleteID = p.ID
				return m, m.setError(errConfirmDelete)
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
		if m.expiredUserID != "" {
			return m.resumeAfterReLogin(msg.User)
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
		// Keep listening for the next transition before doing anything else,
		// or a single drop would be the last one ever reported.
		cmds = append(cmds, ListenWSState(m.wsClient))

		if msg.Unauthorized {
			m.wsConnected = false
			updated, cmd := m.handleAuthExpired()
			return updated, tea.Batch(append(cmds, cmd)...)
		}

		wasConnected := m.wsConnected
		m.wsConnected = msg.Connected

		// Events were dropped while the socket stayed up, so the view is
		// stale with nothing else to reveal it. Same remedy as a reconnect:
		// re-read the channel. Handled before the transition checks, which
		// would otherwise see no change and do nothing.
		if msg.Desynced {
			if m.activeChan != nil {
				cmds = append(cmds, FetchPosts(m.client, m.activeChan.ID, 0, historyPageSize))
			}
			cmds = append(cmds, m.setError(errDesynced))
			return m, tea.Batch(cmds...)
		}

		if msg.Connected && !wasConnected {
			// Events that arrived while the socket was down are gone for
			// good — the stream has no replay — so re-read the channel
			// rather than leaving a silent hole in the history.
			if m.activeChan != nil {
				cmds = append(cmds, FetchPosts(m.client, m.activeChan.ID, 0, historyPageSize))
			}
			cmds = append(cmds, m.setError(errReconnected))
		}
		if !msg.Connected && wasConnected {
			cmds = append(cmds, m.setError(errDisconnected))
		}
		return m, tea.Batch(cmds...)

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
		trimmed := strings.TrimSpace(msg.Input)

		// Split off the verb so commands that take arguments are matched the
		// same way as the ones that do not. "/leave now" is still /leave.
		verb, args, _ := strings.Cut(trimmed, " ")
		args = strings.TrimSpace(args)

		// These act on this client alone; the server knows nothing about them.
		switch verb {
		case "/skin", "/theme":
			m.skinPicker.SetSkins(theme.ListAvailable())
			cmd := m.setFocus(FocusSkinPicker)
			m.skinPicker.Open()
			return m, cmd
		case "/logout":
			return m.handleLogout()
		case "/group":
			// Client-local for the same reason as /leave: the server has no
			// group command, only the REST endpoint the picker's result calls.
			m.pendingGroupChannel = true
			m.pendingPrivateChannel = nil
			cmd := m.setFocus(FocusDMPicker)
			m.dmPicker.OpenForMembers()
			return m, cmd
		case "/threads":
			if m.activeTeam == nil {
				return m, m.setError(errNoTeam)
			}
			cmd := m.setFocus(FocusThreadInbox)
			m.threadInbox.SetChannelNames(m.channelDisplayNames())
			m.threadInbox.Open()
			return m, tea.Batch(cmd, FetchMyThreads(m.client, m.activeTeam.ID))
		case "/leave":
			// Handled here rather than server-side: there is no leave command
			// in the registry, and the REST endpoint already permits a member
			// to remove themselves.
			if m.activeChan == nil || m.me == nil {
				return m, nil
			}
			return m, LeaveChannel(m.client, m.activeChan.ID, m.me.ID)
		case "/nick":
			// Display name only. The handle is /username, kept separate
			// because renaming it breaks every @mention already written.
			if args == "" {
				return m, m.setError(errNickUsage)
			}
			return m, UpdateProfile(m.client, &model.User{DisplayName: args})
		case "/username":
			if args == "" {
				return m, m.setError(errUsernameUsage)
			}
			if strings.ContainsAny(args, " \t") {
				return m, m.setError(errUsernameSpaces)
			}
			return m, UpdateProfile(m.client, &model.User{Username: args})
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
			// The same fields UserLoadedMsg sets. A username change decides
			// which @mentions highlight, and the cached copy in m.users is
			// what every post's author line is rendered from — leaving either
			// stale shows the old name until the next sign-in.
			m.me = msg.User
			m.users[msg.User.ID] = msg.User
			m.viewport.SetCurrentUsername(msg.User.Username)
			m.thread.SetCurrentUsername(msg.User.Username)
			m.resolveDMDisplayNames()
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
	// The tag picker acts on the post under the cursor or the open thread's
	// root, so it returns to whichever pane that was. The history pane is
	// hidden behind an open thread, and keys sent there act on posts the
	// reader cannot see.
	tagPickerReturn := FocusViewport
	if m.mainPane == paneThread {
		tagPickerReturn = FocusThread
	}
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
			visible: func() bool { return m.threadInbox.Visible() },
			update: func(msg tea.KeyMsg) tea.Cmd {
				var cmd tea.Cmd
				m.threadInbox, cmd = m.threadInbox.Update(msg)
				return cmd
			},
			view:       func() string { return m.threadInbox.View() },
			blur:       m.threadInbox.Blur,
			setSize:    m.threadInbox.SetSize,
			setStyles:  m.threadInbox.SetStyles,
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
			closeFocus: tagPickerReturn,
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
	m.actionBar.SetEditing(m.editingPostID != "")

	// Replying needs a focused history pane with a post under the cursor;
	// the button appears only then, which is also the hint that the pane has
	// to be focused first.
	m.actionBar.SetCanReply(m.mainPane == paneChannel &&
		m.focus == FocusViewport && m.viewport.SelectedPost() != nil)
}

func (m *Model) setFocus(area FocusArea) tea.Cmd {
	// Blurring the pane about to be focused again would redraw it twice for
	// nothing; the history pane redraws its whole content to do it.
	if area != FocusViewport {
		m.viewport.Blur()
	}
	if area != FocusInput {
		m.input.Blur()
	}
	if area != FocusThread {
		m.thread.Blur()
	}
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
	case FocusThreadInbox:
		m.threadInbox.Focus()
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
	m.cancelEdit()
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
	// Paging state belongs to the channel being left.
	m.historyPage, m.loadingOlder, m.historyExhausted = 0, false, false
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

// tagByName finds a known tag, ignoring case.
func (m Model) tagByName(name string) *model.Tag {
	for _, t := range m.allTags {
		if strings.EqualFold(t.Name, name) {
			return t
		}
	}
	return nil
}

// tagPost applies tags by name to a post, creating any that do not exist yet.
func (m Model) tagPost(postID string, names []string) []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(names))
	for _, name := range names {
		if t := m.tagByName(name); t != nil {
			cmds = append(cmds, AddTagToPostCmd(m.client, postID, t.ID))
		} else {
			cmds = append(cmds, CreateTagAndApplyCmd(m.client, name, postID))
		}
	}
	return cmds
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
			// Reply counts are left to thread_updated, which carries the
			// server's total; counting here as well over-counted each reply.
			if p.RootID != "" && m.mainPane == paneThread && p.RootID == m.threadRootID {
				m.thread.AppendReply(p) // ignores a reply already shown
			}
			// The sender already appended this from the HTTP response.
			if m.viewport.HasPost(p.ID) {
				return m, tea.Batch(cmds...)
			}
			m.viewport.AppendPost(p)
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
				cmds = append(cmds, m.leaveThread())
			} else if m.mainPane == paneThread {
				m.thread.RemoveReply(postID)
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
			author := m.commandAuthor(slug)
			// Each response needs its own ID: the render cache is keyed by
			// it, so a reused one showed the first output again.
			m.commandResponses++
			m.viewport.AppendPost(&model.Post{
				ID:        fmt.Sprintf("%s:%d", author, m.commandResponses),
				ChannelID: channelID,
				UserID:    author,
				Content:   text,
				CreateAt:  time.Now().UnixMilli(),
			})
			m.viewport.SetUsernames(m.usernameMap())
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

// fetchMissingUsers looks up the authors still unknown. Each is asked for
// once: one the server does not return (a deleted user) stays unknown,
// rather than being asked for again on every post, page and DM load.
func (m Model) fetchMissingUsers() tea.Cmd {
	var missing []string
	for id, u := range m.users {
		if u == nil && !m.usersRequested[id] {
			m.usersRequested[id] = true
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return FetchUsersByIDs(m.client, missing)
}

// usernameMap names every author known so far: real users that have loaded
// (lookups still in flight are nil and skipped) and command pseudo-authors.
func (m Model) usernameMap() map[string]string {
	names := make(map[string]string, len(m.users)+len(m.commandAuthors))
	for id, u := range m.users {
		if u != nil {
			names[id] = u.Username
		}
	}
	maps.Copy(names, m.commandAuthors)
	return names
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

// commandResponseUserID prefixes the pseudo-authors of ephemeral command
// output. It is not a real user and is never looked up.
const commandResponseUserID = "chit:command-response"

// commandAuthor registers the pseudo-author for a command's output and
// returns its user ID. The author is named after the command, so a reply
// reads as coming from "/help" rather than from whoever typed it, and each
// command gets its own so earlier output keeps its name.
func (m *Model) commandAuthor(slug string) string {
	name := "/" + slug
	if slug == "" {
		name = "command"
	}
	id := commandResponseUserID + ":" + name
	m.commandAuthors[id] = name
	return id
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

// channelDisplayNames maps channel IDs to the names shown in the UI, so the
// thread inbox can say where each thread lives.
func (m Model) channelDisplayNames() map[string]string {
	names := make(map[string]string, len(m.channels)+len(m.dmChannels))
	for _, ch := range m.channels {
		names[ch.ID] = ch.DisplayName
	}
	for _, ch := range m.dmChannels {
		if name, ok := m.dmDisplayNames[ch.ID]; ok && name != "" {
			names[ch.ID] = name
			continue
		}
		names[ch.ID] = ch.DisplayName
	}
	return names
}
