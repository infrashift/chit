package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/config"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
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
	return []*model.Command{
		{Slug: "theme", Description: "choose a theme (alias: /skin)"},
		{Slug: "group", Description: "start a group conversation with three or more people"},
		{Slug: "nick", Description: "change your display name"},
		{Slug: "username", Description: "change your username (breaks existing @mentions)"},
		{Slug: "threads", Description: "threads you follow, in this team and in DMs"},
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

// handleSlash runs a slash command typed into the input. Commands this
// client implements are handled here; the rest go to the server.
func (m Model) handleSlash(msg input.SlashTriggerMsg) (tea.Model, tea.Cmd) {
	trimmed := strings.TrimSpace(msg.Input)

	// Split off the verb so commands that take arguments are matched the
	// same way as the ones that do not. "/leave now" is still /leave.
	verb, args, _ := strings.Cut(trimmed, " ")
	args = strings.TrimSpace(args)

	// These act on this client alone; the server knows nothing about them.
	switch verb {
	case "/skin", "/theme":
		m.skinPicker.SetSkins(theme.ListAvailable(config.ThemesDir()))
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
		return m, tea.Batch(cmd, FetchMyThreads(m.reqCtx(), m.client, teamID))
	case "/leave":
		// Handled here rather than server-side: there is no leave command
		// in the registry, and the REST endpoint already permits a member
		// to remove themselves.
		if m.activeChan == nil || m.me == nil {
			return m, nil
		}
		return m, LeaveChannel(m.reqCtx(), m.client, m.activeChan.ID, m.me.ID)
	case "/nick":
		// Display name only. The handle is /username, kept separate
		// because renaming it breaks every @mention already written.
		if args == "" {
			return m, m.setError(errNickUsage)
		}
		return m, UpdateProfile(m.reqCtx(), m.client, &model.User{DisplayName: args})
	case "/username":
		if args == "" {
			return m, m.setError(errUsernameUsage)
		}
		if strings.ContainsAny(args, " \t") {
			return m, m.setError(errUsernameSpaces)
		}
		return m, UpdateProfile(m.reqCtx(), m.client, &model.User{Username: args})
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
	return m, CreatePost(m.reqCtx(), m.client, &model.Post{
		ChannelID: m.activeChan.ID,
		UserID:    m.me.ID,
		Content:   trimmed,
	})
}
