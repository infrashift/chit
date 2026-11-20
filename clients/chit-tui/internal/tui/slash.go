package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/palette"
)

var (
	errNickUsage      = errors.New("usage: /nick <display name>")
	errUsernameUsage  = errors.New("usage: /username <handle>")
	errUsernameSpaces = errors.New("a username cannot contain spaces — try /nick for a display name")
	// Not a failure: setError is the only status-bar channel there is.
	errProfileSaved = errors.New("profile updated")
)

// clientCommands are handled entirely by this client and never reach the
// server, so the server's registry does not list them. Typing one directly has
// always worked; browsing for one had not, because a bare "/" opens the
// palette on the server's list alone and these were absent from it.
func clientCommands() []*model.Command {
	cmds := []*model.Command{
		{Slug: "theme", Description: "choose a theme; /theme <name>, /theme reload (alias: /skin)"},
		{Slug: "chtag", Description: "channel tags: /chtag add <name>, rm <name>, find <name>…"},
		{Slug: "group", Description: "start a group conversation with three or more people"},
		{Slug: "nick", Description: "change your display name"},
		{Slug: "username", Description: "change your username (breaks existing @mentions)"},
		{Slug: "threads", Description: "threads you follow, in this team and in DMs"},
		{Slug: "leave", Description: "leave the current channel"},
		{Slug: "logout", Description: "sign out and clear the stored session"},
	}
	for _, c := range cmds {
		assert.That(c.Slug != "", "clientCommands: command without a slug")
	}
	assert.That(len(cmds) > 0, "clientCommands: no client commands")
	return cmds
}

// mergeCommands appends the server's commands to the client's, dropping any
// the client already handles: a server entry of the same name would be routed
// to a handler that never runs.
func mergeCommands(local, remote []*model.Command) []*model.Command {
	seen := make(map[string]bool, len(local))
	for _, c := range local {
		assert.That(c != nil, "mergeCommands: nil local command")
		seen[c.Slug] = true
	}
	merged := append([]*model.Command(nil), local...)
	for _, c := range remote {
		if c != nil && !seen[c.Slug] {
			merged = append(merged, c)
		}
	}
	assert.That(len(merged) >= len(local), "mergeCommands: a local command was dropped")
	assert.That(len(merged) <= len(local)+len(remote), "mergeCommands: more commands than given")
	return merged
}

// handleSlash runs a slash command typed into the input. Commands this
// client implements are handled here; the rest go to the server.
func (m Model) handleSlash(msg input.SlashTriggerMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleSlash: not signed in")
	assert.That(m.client != nil, "handleSlash: nil client")
	assert.That(m.scope != nil, "handleSlash: nil request scope")
	trimmed := strings.TrimSpace(msg.Input)

	// Split off the verb so commands that take arguments are matched the
	// same way as the ones that do not. "/leave now" is still /leave.
	verb, args, hasArgs := strings.Cut(trimmed, " ")
	if hasArgs {
		args = strings.TrimSpace(args)
	}

	if next, cmd, ok := m.runClientCommand(verb, args); ok {
		return next, cmd
	}

	// Everything else goes to the server, which owns the command
	// registry, authorization, and the "unknown command" reply. Anything
	// that is not a command — a path like /usr/local/bin — is persisted
	// as an ordinary message.
	if m.activeChan == nil || m.me == nil {
		return m, nil
	}
	return m, CreatePost(m.reqCtx(), m.client, &model.Post{
		ChannelID: m.activeChan.ID,
		UserID:    m.me.ID,
		Content:   trimmed,
	})
}

// runClientCommand runs verb if this client implements it, and reports
// whether it did.
func (m Model) runClientCommand(verb, args string) (tea.Model, tea.Cmd, bool) {
	assert.That(m.client != nil, "runClientCommand: nil client")
	assert.That(m.scope != nil, "runClientCommand: nil request scope")
	assert.That(m.appState == AppStateRunning, "runClientCommand: not signed in")
	// These act on this client alone; the server knows nothing about them.
	switch verb {
	case "/skin", "/theme":
		return handled(m.themeCommand(args))
	case "/chtag":
		return handled(m.chtagCommand(args))
	case "/logout":
		return handled(m.handleLogout())
	case "/group":
		// Client-local for the same reason as /leave: the server has no
		// group command, only the REST endpoint the picker's result calls.
		m.pendingGroupChannel = true
		m.pendingPrivateChannel = nil
		cmd := m.setFocus(FocusDMPicker)
		m.dmPicker.OpenForMembers()
		assert.That(m.focus == FocusDMPicker, "runClientCommand: member picker not focused")
		return m, cmd, true
	case "/threads":
		return handled(m.openThreadInbox())
	case "/leave":
		// Handled here rather than server-side: there is no leave command
		// in the registry, and the REST endpoint already permits a member
		// to remove themselves.
		if m.activeChan == nil || m.me == nil {
			return m, nil, true
		}
		return m, LeaveChannel(m.reqCtx(), m.client, m.activeChan.ID, m.me.ID), true
	case "/nick":
		return handled(m.setNick(args))
	case "/username":
		return handled(m.setUsername(args))
	case "/":
		// A bare slash is a request to browse, not to send.
		return m, m.openPalette("/"), true
	}
	return m, nil, false
}

// openThreadInbox opens the list of threads the user follows.
func (m Model) openThreadInbox() (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "openThreadInbox: nil request scope")
	assert.That(m.appState == AppStateRunning, "openThreadInbox: not signed in")
	assert.That(m.client != nil, "openThreadInbox: nil client")
	// No team is no reason to refuse: DM and group threads belong
	// to none, and are listed regardless.
	teamID, teamName := "", ""
	if m.activeTeam != nil {
		teamID, teamName = m.activeTeam.ID, m.activeTeam.DisplayName
	}
	cmd := m.setFocus(FocusThreadInbox)
	m.threadInbox.SetChannelNames(m.channelDisplayNames())
	m.threadInbox.SetTeamName(teamName)
	m.threadInbox.Open()
	assert.That(m.focus == FocusThreadInbox, "openThreadInbox: inbox not focused")
	assert.That(m.threadInbox.Visible(), "openThreadInbox: inbox not open")
	return m, tea.Batch(cmd, FetchMyThreads(m.reqCtx(), m.client, teamID))
}

// setNick changes the display name only. The handle is /username, kept
// separate because renaming it breaks every @mention already written.
func (m Model) setNick(args string) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "setNick: nil request scope")
	assert.That(m.appState == AppStateRunning, "setNick: not signed in")
	assert.That(m.client != nil, "setNick: nil client")
	if args == "" {
		return m, m.setError(errNickUsage)
	}
	return m, UpdateProfile(m.reqCtx(), m.client, &model.User{DisplayName: args})
}

// setUsername changes the handle others @mention.
func (m Model) setUsername(args string) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "setUsername: nil request scope")
	assert.That(m.appState == AppStateRunning, "setUsername: not signed in")
	assert.That(m.client != nil, "setUsername: nil client")
	if args == "" {
		return m, m.setError(errUsernameUsage)
	}
	if strings.ContainsAny(args, " \t") {
		return m, m.setError(errUsernameSpaces)
	}
	return m, UpdateProfile(m.reqCtx(), m.client, &model.User{Username: args})
}

func (m Model) handleCommandsLoaded(msg CommandsLoadedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleCommandsLoaded: not signed in")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	m.palette.SetCommands(mergeCommands(clientCommands(), msg.Commands))
	return m, nil
}

func (m Model) handleCommandChosen(msg palette.CommandChosenMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleCommandChosen: not signed in")
	// Insert rather than execute: most commands take arguments, and the
	// palette has no way to collect them. The user completes the line and
	// presses Enter, which sends it like any other message.
	if msg.Command != nil {
		m.input.SetValue("/" + msg.Command.Slug + " ")
	}
	return m, m.setFocus(FocusInput)
}
