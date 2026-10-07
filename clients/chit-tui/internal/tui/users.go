package tui

import (
	"maps"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/mention"
)

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
	return FetchUsersByIDs(m.reqCtx(), m.client, missing)
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

// applyMe records the signed-in user, on sign-in and after a profile edit.
// The username decides which @mentions highlight, and the copy in m.users is
// what every post's author line is drawn from; leaving either stale shows
// the old name until the next sign-in.
func (m *Model) applyMe(u *model.User) {
	m.me = u
	m.users[u.ID] = u
	m.viewport.SetCurrentUsername(u.Username)
	m.thread.SetCurrentUsername(u.Username)
	// The panes only learn names from here or from a user lookup, and the
	// signed-in user is never looked up, so their own posts showed their ID
	// until some other author happened to load.
	m.viewport.SetUsernames(m.usernameMap())
	m.thread.SetUsernames(m.usernameMap())
	m.resolveDMDisplayNames()
}

// handleUserSearchResults hands user search results to whichever overlay
// asked, if they answer the latest query.
func (m Model) handleUserSearchResults(msg UserSearchResultsMsg) (tea.Model, tea.Cmd) {
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
}

// handleUsersLoaded records looked-up users and renames everything drawn
// with them.
func (m Model) handleUsersLoaded(msg UsersLoadedMsg) (tea.Model, tea.Cmd) {
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
}

// handleUserLoaded records the signed-in user.
func (m Model) handleUserLoaded(msg UserLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	if m.expiredUserID != "" {
		return m.resumeAfterReLogin(msg.User)
	}
	firstLoad := m.me == nil
	m.applyMe(msg.User)
	// Member rows that arrived before the user did could not be
	// matched to them; work their badges out now.
	if firstLoad {
		for channelID, members := range m.channelMembers {
			m.applyMembers(channelID, members)
		}
	}
	return m, m.fetchMissingUsers()
}
