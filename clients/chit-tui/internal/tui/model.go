package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/confirm"
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
	// The active theme and its runtime state (themes.go).
	themeKey  string
	themePath string
	themeMod  time.Time
	// previewFrom is the theme before the first preview of a picker
	// session, or nil when no preview is on screen.
	previewFrom *themeState
	// themeCache holds the themes resolved during one picker session, so
	// a preview does not read a file for each cursor move.
	themeCache map[string]theme.Theme
	themeWatch bool
	themePaint bool
	// watchSeq names the one live theme-file watch loop (themes.go).
	watchSeq uint64
	// clipboardOut is where copying writes its OSC 52 sequence: the
	// terminal, except in tests, which would otherwise write escape
	// sequences into their output.
	clipboardOut io.Writer
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
	// confirm asks the y/N questions: a post delete, a tag delete. Its
	// subjects are in bubbles.go.
	confirm confirm.Model
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
	assert.That(client != nil, "NewModel: nil client")
	state := AppStateRunning
	if tokenStore != nil && tokenStore.Get() == "" {
		state = AppStateLogin
	}
	if sessionStore == nil {
		sessionStore = auth.NewSessionStore("")
	}

	m := Model{
		scope:        newRequestScope(),
		cfg:          cfg,
		client:       client,
		wsClient:     wsClient,
		viewport:     viewport.New(s),
		input:        input.New(s),
		actionBar:    actionbar.New(s),
		thread:       thread.New(s),
		palette:      palette.New(s),
		threadInbox:  threadinbox.New(s),
		help:         help.New(s),
		mention:      mention.New(s),
		dmPicker:     dmpicker.New(s),
		skinPicker:   skinpicker.New(s),
		chCreator:    chcreator.New(s),
		tagPicker:    tagpicker.New(s),
		confirm:      confirm.New(s, questionTTL),
		loginModel:   login.New(s, kratosClient),
		themeSetting: "theme",
		themeWatch:   cfg != nil && cfg.ThemeWatch,
		themePaint:   cfg != nil && cfg.ThemePaint,
		clipboardOut: os.Stdout,
		appState:     state,
		tokenStore:   tokenStore,
		kratosClient: kratosClient,
		sessionStore: sessionStore,
		focus:        FocusInput,
		keys:         DefaultKeyMap(),
		styles:       s,
	}
	m.makeCaches()
	// Seed the client-local commands rather than waiting for the server
	// registry, which never contains them and may never arrive at all.
	m.palette.SetCommands(clientCommands())

	// The palette shares the root's badge and DM-name maps by reference, so
	// root-side updates are visible without further plumbing.
	m.palette.SetCounts(m.unread, m.mentions)
	m.palette.SetDMDisplayNames(m.dmDisplayNames)
	// Focus also returns the first blink, but NewModel cannot run a
	// command. applyLoadedUser starts the blink when the session starts.
	firstBlink := m.input.Focus()
	assert.That(firstBlink != nil, "NewModel: focused input without a blink")
	assert.That(m.scope != nil, "NewModel: nil request scope")
	assert.That(m.sessionStore != nil, "NewModel: nil session store")
	assert.That(m.appState != AppStateReLogin, "NewModel: a new model starts in re-login")
	assert.That(m.focus == FocusInput, "NewModel: input does not start focused")
	return m
}

// makeCaches gives a new model the empty maps its session fills in.
func (m *Model) makeCaches() {
	assert.That(m.users == nil, "makeCaches: caches already made")
	assert.That(m.unread == nil, "makeCaches: badge caches already made")
	m.users = make(map[string]*model.User)
	m.channelsByTeam = make(map[string][]*model.Channel)
	m.unread = make(map[string]int64)
	m.mentions = make(map[string]int64)
	m.dmDisplayNames = make(map[string]string)
	m.channelMembers = make(map[string][]*model.ChannelMember)
	m.threadCounts = make(map[string]int)
	m.postTags = make(map[string][]*model.Tag)
	m.commandAuthors = make(map[string]string)
	m.usersRequested = make(map[string]bool)
	assert.That(len(m.users) == 0, "makeCaches: users cache not empty")
	assert.That(len(m.commandAuthors) == 0, "makeCaches: command authors not empty")
}

// SetThemeSetting sets the config key a theme picked with /theme is saved
// under, which depends on the appearance resolved at startup.
func (m *Model) SetThemeSetting(key string) {
	assert.That(key != "", "SetThemeSetting: empty key")
	m.themeSetting = key
}

// setError sets the error and returns a command to auto-clear it after 10 seconds.
func (m *Model) setError(err error) tea.Cmd {
	assert.That(err != nil, "setError: nil error")
	m.errSeq++
	assert.That(m.errSeq > 0, "setError: error sequence wrapped")
	m.err = err
	// The status line shows one thing. An error pushes an open question
	// off it, and a key must not answer a question that is not shown.
	m.confirm.Cancel()
	seq := m.errSeq
	return tea.Tick(10*time.Second, func(_ time.Time) tea.Msg {
		return ClearErrMsg{Seq: seq}
	})
}

// Init returns the initial commands.
func (m Model) Init() tea.Cmd {
	assert.That(m.scope != nil, "Init: nil request scope")
	assert.That(m.appState >= AppStateLogin, "Init: unknown app state")
	assert.That(m.appState <= AppStateRunning, "Init: app state past running")
	// The terminal takes the theme's colors before anything else draws.
	paint := m.paintCmd()
	if m.appState == AppStateLogin || m.appState == AppStateReLogin {
		return paint
	}
	return tea.Batch(paint, m.watchTick(m.watchSeq), m.initRunning())
}

func (m Model) initRunning() tea.Cmd {
	assert.That(m.appState == AppStateRunning, "initRunning: not signed in")
	assert.That(m.client != nil, "initRunning: nil client")
	assert.That(m.scope != nil, "initRunning: nil request scope")
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
	assert.That(len(cmds) >= 5, "initRunning: a startup request is missing")
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
	assert.That(ctx.Err() == nil, "newRequestScope: new context already canceled")
	assert.That(cancel != nil, "newRequestScope: nil cancel")
	return &requestScope{ctx: ctx, cancel: cancel}
}

// reqCtx is the context requests made now should run under.
func (m Model) reqCtx() context.Context {
	assert.That(m.scope != nil, "reqCtx: nil request scope")
	assert.That(m.scope.ctx != nil, "reqCtx: nil context")
	return m.scope.ctx
}

// endRequests cancels every request of the session that is ending and
// starts a fresh scope for the next one.
func (m *Model) endRequests() {
	assert.That(m.scope != nil, "endRequests: nil request scope")
	old := m.scope
	m.scope.cancel()
	m.scope = newRequestScope()
	assert.That(old.ctx.Err() != nil, "endRequests: the old scope is still live")
	assert.That(m.scope != old, "endRequests: the scope was not replaced")
}

func (m Model) handleErr(msg ErrMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.Err != nil, "handleErr: nil error")
	assert.That(m.appState == AppStateRunning, "handleErr: not signed in")
	if errors.Is(msg.Err, ws.ErrNotConnected) {
		m.wsConnected = false
	}
	return m, m.setError(msg.Err)
}
