package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	// themeSetting is the config key a theme picked in the client is saved
	// under; see config.Config.ThemeSettingKey.
	themeSetting string
	// usersRequested marks user IDs already looked up.
	usersRequested map[string]bool
	// commandAuthors names the pseudo-author of each command's output, by
	// user ID; commandResponses numbers the outputs so each has its own ID.
	commandAuthors   map[string]string
	commandResponses int
	// scope is the signed-in session's request context. Every request runs
	// under it, so ending the session cancels whatever is still in flight.
	scope *requestScope
	// pendingJumpID is a search hit to select once its channel has loaded.
	pendingJumpID string
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
		scope:          newRequestScope(),
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
		themeSetting:   "theme",
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

// SetThemeSetting sets the config key a theme picked with /theme is saved
// under, which depends on the appearance resolved at startup.
func (m *Model) SetThemeSetting(key string) { m.themeSetting = key }

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
		FetchMe(m.reqCtx(), m.client),
		FetchTeams(m.reqCtx(), m.client),
		FetchCommands(m.reqCtx(), m.client),
		FetchDMChannels(m.reqCtx(), m.client),
		FetchAllTags(m.reqCtx(), m.client),
	}
	if m.wsClient != nil {
		cmds = append(cmds, func() tea.Msg {
			return WSConnectedMsg{Err: m.wsClient.Connect()}
		})
	}
	return tea.Batch(cmds...)
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

// requestScope is a cancellable context shared by one session's requests.
type requestScope struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newRequestScope() *requestScope {
	ctx, cancel := context.WithCancel(context.Background())
	return &requestScope{ctx: ctx, cancel: cancel}
}

// reqCtx is the context requests made now should run under.
func (m Model) reqCtx() context.Context { return m.scope.ctx }

// endRequests cancels every request of the session that is ending and
// starts a fresh scope for the next one.
func (m *Model) endRequests() {
	m.scope.cancel()
	m.scope = newRequestScope()
}
