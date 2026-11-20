// Package palette implements the unified floating palette: the single entry
// point for jumping to channels/DMs, finding people, running slash commands,
// and searching messages. The first character of the query selects the mode:
// "@" people, "/" commands, "?" message search, anything else channels.
package palette

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/listwin"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/styles"
)

// Mode is the palette's active result mode, derived from the query prefix.
type Mode int

const (
	ModeChannels Mode = iota // default: jump to a channel or DM
	ModeUsers                // "@": find a person, open a DM
	ModeCommands             // "/": slash commands
	ModeSearch               // "?": full-text search in the active channel
)

// debounceDelay is how long typing must pause before a user search fires.
const debounceDelay = 300 * time.Millisecond

// ChannelChosenMsg is sent when a channel or DM is selected.
type ChannelChosenMsg struct{ Channel *model.Channel }

// CommandChosenMsg is sent when a slash command is selected.
type CommandChosenMsg struct{ Command *model.Command }

// UserChosenMsg is sent when a person is selected in "@" mode.
type UserChosenMsg struct{ User *model.User }

// UserQueryMsg asks the app to run a user search for the term.
type UserQueryMsg struct{ Term string }

// SearchSubmitMsg asks the app to run a message search for the term.
type SearchSubmitMsg struct {
	Term string
	// Everywhere widens the search past the active channel. A message you
	// half-remember is often in a channel you cannot name, so "?" searches
	// here and "??" searches everywhere.
	Everywhere bool
}

// PostChosenMsg is sent when a search result is selected.
type PostChosenMsg struct{ Post *model.Post }

// DebounceMsg is the internal typing-pause timer; the app must route it back
// into Update.
type DebounceMsg struct{ Gen int }

// row is one selectable result line.
type row struct {
	label   string
	channel *model.Channel
	user    *model.User
	command *model.Command
	post    *model.Post
}

// Model is the unified palette overlay component.
type Model struct {
	input          textinput.Model
	mode           Mode
	channels       []*model.Channel
	dms            []*model.Channel
	teamNames      map[string]string // teamID -> display name
	multiTeam      bool
	dmNames        map[string]string // channelID -> DM display name (shared with root)
	unread         map[string]int64  // shared with root
	mentions       map[string]int64  // shared with root
	commands       []*model.Command
	users          []*model.User
	posts          []*model.Post
	usernames      map[string]string
	activeChanName string
	rows           []row
	cursor         int
	debounceGen    int
	visible        bool
	focused        bool
	styles         styles.Styles
	width          int
	height         int
}

// New creates a new palette.
func New(s styles.Styles) Model {
	ti := textinput.New()
	ti.Placeholder = "Jump to channel — @ people, / commands, ? search"
	ti.CharLimit = 256
	m := Model{
		input:     ti,
		teamNames: make(map[string]string),
		dmNames:   make(map[string]string),
		unread:    make(map[string]int64),
		mentions:  make(map[string]int64),
		usernames: make(map[string]string),
		styles:    s,
	}
	assert.That(m.mode == modeFor(m.input.Value()), "palette.New: mode out of step with the query")
	assert.That(!m.visible && !m.focused, "palette.New: a new palette starts hidden")
	return m
}

// SetChannels sets the flattened all-team channel list.
func (m *Model) SetChannels(channels []*model.Channel) {
	m.channels = channels
	m.refresh()
	assert.That(m.mode == modeFor(m.input.Value()), "palette.SetChannels: mode out of step with the query")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.SetChannels: cursor outside the rows")
	assert.That(m.mode != ModeChannels || len(m.rows) <= len(m.channels)+len(m.dms),
		"palette.SetChannels: more rows than channels")
}

// SetDMChannels sets the DM/group channel list.
func (m *Model) SetDMChannels(channels []*model.Channel) {
	m.dms = channels
	m.refresh()
	assert.That(m.mode == modeFor(m.input.Value()), "palette.SetDMChannels: mode out of step with the query")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.SetDMChannels: cursor outside the rows")
	assert.That(m.mode != ModeChannels || len(m.rows) <= len(m.channels)+len(m.dms),
		"palette.SetDMChannels: more rows than channels")
}

// SetTeams sets the known teams, used for "#channel · team" row suffixes.
func (m *Model) SetTeams(teams []*model.Team) {
	m.teamNames = make(map[string]string, len(teams))
	for _, t := range teams {
		name := t.DisplayName
		if name == "" {
			name = t.Name
		}
		m.teamNames[t.ID] = name
	}
	m.multiTeam = len(teams) > 1
	m.refresh()
	assert.That(len(m.teamNames) <= len(teams), "palette.SetTeams: more team names than teams")
	assert.That(m.mode == modeFor(m.input.Value()), "palette.SetTeams: mode out of step with the query")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.SetTeams: cursor outside the rows")
}

// SetDMDisplayNames shares the root's DM display-name map. The map is held by
// reference, so later root-side updates are picked up automatically.
func (m *Model) SetDMDisplayNames(names map[string]string) {
	assert.That(names != nil, "palette.SetDMDisplayNames: nil map")
	m.dmNames = names
}

// SetCounts shares the root's unread and mention maps by reference.
func (m *Model) SetCounts(unread, mentions map[string]int64) {
	assert.That(unread != nil, "palette.SetCounts: nil unread map")
	assert.That(mentions != nil, "palette.SetCounts: nil mentions map")
	m.unread = unread
	m.mentions = mentions
}

// SetCommands sets the available slash commands.
func (m *Model) SetCommands(cmds []*model.Command) {
	m.commands = cmds
	m.refresh()
	assert.That(m.mode != ModeCommands || len(m.rows) <= len(cmds),
		"palette.SetCommands: more rows than commands")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.SetCommands: cursor outside the rows")
}

// SetUsers sets the "@" mode search results.
func (m *Model) SetUsers(users []*model.User) {
	m.users = users
	m.refresh()
	assert.That(m.mode != ModeUsers || len(m.rows) == len(users),
		"palette.SetUsers: one row per user")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.SetUsers: cursor outside the rows")
}

// SetSearchResults sets the "?" mode search results.
func (m *Model) SetSearchResults(posts []*model.Post) {
	m.posts = posts
	m.refresh()
	assert.That(m.mode != ModeSearch || len(m.rows) == len(posts),
		"palette.SetSearchResults: one row per post")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.SetSearchResults: cursor outside the rows")
}

// SetUsernames sets the userID -> username map used to render search results.
func (m *Model) SetUsernames(names map[string]string) {
	assert.That(names != nil, "palette.SetUsernames: nil map")
	m.usernames = names
}

// SetActiveChannel sets the display name of the active channel, shown in the
// "?" mode header (message search is scoped to the active channel).
func (m *Model) SetActiveChannel(name string) {
	assert.That(m.mode == modeFor(m.input.Value()), "palette.SetActiveChannel: mode out of step with the query")
	m.activeChanName = name
}

// Open shows the palette with the query pre-filled (e.g. "", "@", "/", "?").
func (m *Model) Open(prefix string) {
	m.visible = true
	m.focused = true
	m.input.SetValue(prefix)
	m.input.CursorEnd()
	m.input.Focus()
	m.users = nil
	m.posts = nil
	m.cursor = 0
	m.refresh()
	assert.That(m.mode == modeFor(prefix), "palette.Open: mode does not match the prefix")
	assert.That(m.cursor == 0, "palette.Open: cursor not on the first row")
	assert.That(m.visible && m.focused, "palette.Open: not shown and focused")
	assert.That(m.input.Focused(), "palette.Open: input not focused")
}

// Close hides the palette.
func (m *Model) Close() {
	m.visible = false
	m.focused = false
	m.input.Blur()
	m.users = nil
	m.posts = nil
	assert.That(!m.input.Focused(), "palette.Close: input still focused")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.Close: cursor outside the rows")
}

// Visible returns the visibility state.
func (m Model) Visible() bool { return m.visible }

// Focused returns the focus state.
func (m Model) Focused() bool { return m.focused }

// Focus sets focus.
func (m *Model) Focus() {
	m.focused = true
	m.input.Focus()
	assert.That(m.input.Focused(), "palette.Focus: input not focused")
}

// Blur removes focus.
func (m *Model) Blur() {
	m.focused = false
	m.input.Blur()
	assert.That(!m.input.Focused(), "palette.Blur: input still focused")
}

// SetStyles replaces the styles.
func (m *Model) SetStyles(s styles.Styles) { m.styles = s }

// SetSize sets the available screen dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = w/2 - 8
	assert.That(m.maxVisibleRows() >= 5, "palette.SetSize: fewer than 5 visible rows")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.SetSize: cursor outside the rows")
}

// Mode returns the current mode.
func (m Model) Mode() Mode {
	assert.That(m.mode == modeFor(m.input.Value()), "palette.Mode: mode out of step with the query")
	return m.mode
}

// query returns the input value without its mode prefix.
func (m Model) query() string {
	v := m.input.Value()
	if v == "" {
		return ""
	}
	q := v
	switch v[0] {
	case '@', '/', '?':
		q = v[1:]
	}
	assert.That(len(q) <= len(v), "palette.query: query longer than the input")
	assert.That(len(v)-len(q) <= 1, "palette.query: more than the one-byte prefix removed")
	return q
}

func modeFor(value string) Mode {
	if value == "" {
		return ModeChannels
	}
	switch value[0] {
	case '@':
		return ModeUsers
	case '/':
		return ModeCommands
	case '?':
		return ModeSearch
	}
	return ModeChannels
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if deb, ok := msg.(DebounceMsg); ok {
		return m, m.debounced(deb)
	}

	if !m.visible || !m.focused {
		return m, nil
	}
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.Update: cursor outside the rows")
	assert.That(m.mode == modeFor(m.input.Value()), "palette.Update: mode out of step with the query")

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if next, cmd, handled := m.handleKey(keyMsg); handled {
			return next, cmd
		}
	}

	prevValue := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prevValue {
		cmd = m.queryChanged(cmd)
		assert.That(m.mode == modeFor(m.input.Value()), "palette.Update: mode out of step with the query")
		return m, cmd
	}
	return m, cmd
}

// debounced fires the user search only if no newer keystroke restarted the
// timer.
func (m Model) debounced(deb DebounceMsg) tea.Cmd {
	if !m.visible || deb.Gen != m.debounceGen || m.mode != ModeUsers {
		return nil
	}
	assert.That(strings.HasPrefix(m.input.Value(), "@"), "palette.debounced: user mode without the @ prefix")
	if term := strings.TrimSpace(m.query()); term != "" {
		return func() tea.Msg { return UserQueryMsg{Term: term} }
	}
	return nil
}

// handleKey handles the keys the palette itself uses, and reports whether
// msg was one of them; the rest go to the input.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEscape:
		m.Close()
		assert.That(!m.visible && !m.focused, "palette.handleKey: still shown after Escape")
		return m, nil, true
	case tea.KeyEnter:
		next, cmd := m.choose(m.cursor)
		return next, cmd, true
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		assert.That(m.cursor >= 0, "palette.handleKey: cursor above the first row")
		return m, nil, true
	case tea.KeyDown:
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
		assert.That(m.cursor == 0 || m.cursor < len(m.rows), "palette.handleKey: cursor below the last row")
		return m, nil, true
	}
	return m, nil, false
}

// queryChanged refilters after an edit to the input, whose own command is
// cmd, and starts the user-search timer in user mode.
func (m *Model) queryChanged(cmd tea.Cmd) tea.Cmd {
	prevMode := m.mode
	// A new filter re-ranks the rows, so the best match is at the top;
	// keeping the old index would point Enter at whatever landed there.
	m.cursor = 0
	m.refresh()
	assert.That(m.mode == modeFor(m.input.Value()), "palette.queryChanged: mode out of step with the query")
	assert.That(m.cursor == 0, "palette.queryChanged: cursor not reset to the best match")
	// Editing a submitted search invalidates its results.
	if m.mode == ModeSearch {
		m.posts = nil
		m.rows = nil
	}
	if m.mode != ModeUsers {
		return cmd
	}
	if prevMode != ModeUsers {
		m.users = nil
		m.rows = nil
	}
	m.debounceGen++
	gen := m.debounceGen
	assert.That(gen > 0, "palette.queryChanged: debounce generation did not advance")
	return tea.Batch(cmd, tea.Tick(debounceDelay, func(time.Time) tea.Msg {
		return DebounceMsg{Gen: gen}
	}))
}

// choose activates the row at idx (Enter or mouse click).
func (m *Model) choose(idx int) (Model, tea.Cmd) {
	assert.That(idx >= 0, "palette.choose: negative row index")
	assert.That(m.mode == modeFor(m.input.Value()), "palette.choose: mode out of step with the query")
	switch m.mode {
	case ModeSearch:
		// No results yet: Enter submits the search.
		if len(m.posts) == 0 {
			term := strings.TrimSpace(m.query())
			if term == "" {
				return *m, nil
			}
			everywhere := strings.HasPrefix(term, "?")
			term = strings.TrimSpace(strings.TrimPrefix(term, "?"))
			if term == "" {
				return *m, nil
			}
			return *m, func() tea.Msg {
				return SearchSubmitMsg{Term: term, Everywhere: everywhere}
			}
		}
	case ModeUsers:
		// No results yet: Enter searches immediately, skipping the debounce.
		if len(m.users) == 0 {
			term := strings.TrimSpace(m.query())
			if term == "" {
				return *m, nil
			}
			return *m, func() tea.Msg { return UserQueryMsg{Term: term} }
		}
	}

	if idx < 0 || idx >= len(m.rows) {
		return *m, nil
	}
	r := m.rows[idx]
	assert.That(r.channel != nil || r.user != nil || r.command != nil || r.post != nil,
		"palette.choose: row has no target")
	m.Close()
	assert.That(!m.visible, "palette.choose: still visible after a choice")
	switch {
	case r.channel != nil:
		ch := r.channel
		return *m, func() tea.Msg { return ChannelChosenMsg{Channel: ch} }
	case r.user != nil:
		u := r.user
		return *m, func() tea.Msg { return UserChosenMsg{User: u} }
	case r.command != nil:
		c := r.command
		return *m, func() tea.Msg { return CommandChosenMsg{Command: c} }
	case r.post != nil:
		p := r.post
		return *m, func() tea.Msg { return PostChosenMsg{Post: p} }
	}
	return *m, nil
}

// refresh recomputes the mode and result rows from the current query.
func (m *Model) refresh() {
	m.mode = modeFor(m.input.Value())
	switch m.mode {
	case ModeChannels:
		m.rows = m.channelRows()
	case ModeUsers:
		m.rows = m.userRows()
	case ModeCommands:
		m.rows = m.commandRows()
	case ModeSearch:
		m.rows = m.postRows()
	}
	if m.cursor >= len(m.rows) {
		m.cursor = 0
	}
	assert.That(m.mode >= ModeChannels && m.mode <= ModeSearch, "palette.refresh: unknown mode")
	assert.That(m.mode != ModeUsers || len(m.rows) == len(m.users), "palette.refresh: one row per user")
	assert.That(m.mode != ModeSearch || len(m.rows) == len(m.posts), "palette.refresh: one row per post")
	assert.That(m.mode != ModeCommands || len(m.rows) <= len(m.commands), "palette.refresh: more rows than commands")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.refresh: cursor outside the rows")
}

// channelDisplayName resolves the label for a channel or DM row.
func (m Model) channelDisplayName(ch *model.Channel) string {
	assert.That(ch != nil, "palette.channelDisplayName: nil channel")
	assert.That(m.dmNames != nil, "palette.channelDisplayName: nil DM name map")
	if name := m.dmNames[ch.ID]; name != "" {
		return name
	}
	if ch.DisplayName != "" {
		return ch.DisplayName
	}
	return ch.Name
}

func (m Model) channelRows() []row {
	assert.That(m.mode == ModeChannels, "palette.channelRows: called outside channel mode")
	assert.That(m.unread != nil && m.mentions != nil, "palette.channelRows: nil count maps")
	query := strings.TrimSpace(m.query())

	type scored struct {
		ch    *model.Channel
		label string
		score int
	}
	var matches []scored
	for _, ch := range append(append([]*model.Channel{}, m.channels...), m.dms...) {
		label := m.channelDisplayName(ch)
		candidate := label
		if team := m.teamNames[ch.TeamID]; team != "" && m.multiTeam {
			label += " · " + team
			candidate = label
		}
		score, ok := fuzzyScore(query, candidate)
		if !ok {
			continue
		}
		matches = append(matches, scored{ch: ch, label: label, score: score})
	}

	// Fuzzy score first (when filtering), then activity: mentions, unread,
	// most recent post.
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if query != "" && a.score != b.score {
			return a.score > b.score
		}
		if m.mentions[a.ch.ID] != m.mentions[b.ch.ID] {
			return m.mentions[a.ch.ID] > m.mentions[b.ch.ID]
		}
		if m.unread[a.ch.ID] != m.unread[b.ch.ID] {
			return m.unread[a.ch.ID] > m.unread[b.ch.ID]
		}
		return a.ch.LastPostAt > b.ch.LastPostAt
	})

	assert.That(len(matches) <= len(m.channels)+len(m.dms), "palette.channelRows: more matches than channels")
	rows := make([]row, len(matches))
	for i, s := range matches {
		assert.That(s.ch != nil, "palette.channelRows: match without a channel")
		assert.That(query != "" || s.score == 0, "palette.channelRows: empty query scored a match")
		label := s.label
		if unread := m.unread[s.ch.ID]; unread > 0 {
			label += " " + m.styles.UnreadBadge.Render(fmt.Sprintf("(%d)", unread))
		}
		if mentions := m.mentions[s.ch.ID]; mentions > 0 {
			label += " " + m.styles.MentionBadge.Render(fmt.Sprintf("[@%d]", mentions))
		}
		rows[i] = row{label: label, channel: s.ch}
	}
	return rows
}

func (m Model) userRows() []row {
	assert.That(m.mode == ModeUsers, "palette.userRows: called outside user mode")
	rows := make([]row, len(m.users))
	for i, u := range m.users {
		label := "@" + u.Username
		if u.DisplayName != "" {
			label += " (" + u.DisplayName + ")"
		}
		rows[i] = row{label: label, user: u}
	}
	assert.That(len(rows) == len(m.users), "palette.userRows: one row per user")
	return rows
}

func (m Model) commandRows() []row {
	assert.That(m.mode == ModeCommands, "palette.commandRows: called outside command mode")
	query := strings.TrimSpace(m.query())

	type scored struct {
		cmd   *model.Command
		score int
	}
	var matches []scored
	for _, c := range m.commands {
		score, ok := fuzzyScore(query, c.Slug+" "+c.Description)
		if !ok {
			continue
		}
		matches = append(matches, scored{cmd: c, score: score})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	assert.That(len(matches) <= len(m.commands), "palette.commandRows: more matches than commands")

	rows := make([]row, len(matches))
	for i, s := range matches {
		assert.That(s.cmd != nil, "palette.commandRows: match without a command")
		rows[i] = row{
			label:   "/" + s.cmd.Slug + "  " + m.styles.Timestamp.Render(s.cmd.Description),
			command: s.cmd,
		}
	}
	assert.That(len(rows) == len(matches), "palette.commandRows: one row per match")
	return rows
}

func (m Model) postRows() []row {
	assert.That(m.mode == ModeSearch, "palette.postRows: called outside search mode")
	assert.That(m.usernames != nil, "palette.postRows: nil username map")
	rows := make([]row, len(m.posts))
	for i, p := range m.posts {
		author := m.usernames[p.UserID]
		if author == "" && p.UserID != "" {
			author = p.UserID[:min(8, len(p.UserID))]
		}
		assert.That(author != "" || p.UserID == "", "palette.postRows: known author left unnamed")
		excerpt := strings.ReplaceAll(p.Content, "\n", " ")
		if len([]rune(excerpt)) > 60 {
			excerpt = string([]rune(excerpt)[:60]) + "…"
		}
		assert.That(len([]rune(excerpt)) <= 61, "palette.postRows: excerpt not truncated")
		rows[i] = row{label: author + ": " + excerpt, post: p}
	}
	return rows
}

// View geometry: inside the bordered+padded box, line 0 is the input, line 1
// is the context line, rows start at contentRowsStart. The box chrome
// (double border + vertical padding) adds chromeTop lines above the content.
// RowAt depends on these staying in sync with View.
const (
	chromeTop        = 2
	contentRowsStart = 2
)

// maxVisibleRows is how many result rows fit in the palette.
func (m Model) maxVisibleRows() int {
	n := max(m.height/2-6, 5)
	assert.That(n >= 5, "palette.maxVisibleRows: below the minimum of 5")
	return n
}

// windowStart is the index of the first visible row, keeping the cursor in view.
func (m Model) windowStart() int {
	start := listwin.Start(m.cursor, len(m.rows), m.maxVisibleRows())
	assert.That(start >= 0, "palette.windowStart: negative start")
	assert.That(start <= m.cursor, "palette.windowStart: cursor above the window")
	assert.That(start == 0 || start < len(m.rows), "palette.windowStart: start past the last row")
	return start
}

// contextLine renders the second line of the palette body.
func (m Model) contextLine() string {
	assert.That(m.visible, "palette.contextLine: rendered while hidden")
	assert.That(m.mode == modeFor(m.input.Value()), "palette.contextLine: mode out of step with the query")
	switch m.mode {
	case ModeSearch:
		if m.activeChanName == "" && !strings.HasPrefix(m.query(), "?") {
			return m.styles.ErrorText.Render(
				"No active channel to search — use ?? to search everywhere")
		}
		scope := m.activeChanName
		if strings.HasPrefix(m.query(), "?") {
			scope = "all channels"
		}
		assert.That(scope != "", "palette.contextLine: empty search scope")
		if len(m.posts) == 0 {
			return m.styles.Timestamp.Render(
				"Search in " + scope + " — press Enter, or ?? to search everywhere")
		}
		return m.styles.Timestamp.Render("Results in " + scope)
	case ModeUsers:
		if len(m.users) == 0 {
			return m.styles.Timestamp.Render("Type a name — Enter to search")
		}
	}
	return ""
}

// footer renders the hint line at the bottom of the palette.
func (m Model) footer() string {
	return m.styles.Timestamp.Render("@ people   / commands   ? search   ↵ select   esc close")
}

// View renders the palette overlay.
func (m Model) View() string {
	if !m.visible {
		return ""
	}

	items := []string{m.input.View(), m.contextLine()}

	start := m.windowStart()
	end := min(start+m.maxVisibleRows(), len(m.rows))
	assert.That(start == 0 || start < end, "palette.View: window starts past its end")
	assert.That(end <= len(m.rows), "palette.View: window ends past the rows")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.View: cursor outside the rows")
	for i := start; i < end; i++ {
		line := m.rows[i].label
		if i == m.cursor {
			line = m.styles.ListItemActive.Render("> " + line)
		} else {
			line = m.styles.ListItem.Render("  " + line)
		}
		items = append(items, line)
	}

	assert.That(len(items) == 2+end-start, "palette.View: one line per visible row")
	items = append(items, "", m.footer())
	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return m.styles.CmdPalette.Width(m.width / 2).Render(content)
}

// RowAt maps a y-offset relative to the top of the rendered overlay to a row
// index, accounting for box chrome and header lines.
func (m Model) RowAt(localY int) (int, bool) {
	idx := localY - chromeTop - contentRowsStart
	if idx < 0 || idx >= m.maxVisibleRows() {
		return 0, false
	}
	idx += m.windowStart()
	if idx >= len(m.rows) {
		return 0, false
	}
	assert.That(idx >= 0, "palette.RowAt: negative row index")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.RowAt: cursor outside the rows")
	return idx, true
}

// ChooseRow activates the row at idx, as if the cursor were on it and Enter
// were pressed.
func (m *Model) ChooseRow(idx int) tea.Cmd {
	updated, cmd := m.choose(idx)
	*m = updated
	assert.That(m.visible || !m.focused, "palette.ChooseRow: hidden but focused")
	assert.That(m.mode == modeFor(m.input.Value()), "palette.ChooseRow: mode out of step with the query")
	assert.That(m.cursor >= 0 && (m.cursor == 0 || m.cursor < len(m.rows)), "palette.ChooseRow: cursor outside the rows")
	return cmd
}

// MoveCursor moves the selection cursor by delta, clamped to the row range.
func (m *Model) MoveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = max(len(m.rows)-1, 0)
	}
	assert.That(m.cursor >= 0, "palette.MoveCursor: cursor above the first row")
	assert.That(m.cursor == 0 || m.cursor < len(m.rows), "palette.MoveCursor: cursor below the last row")
}
