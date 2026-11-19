package tui

import (
	"maps"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/input"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/mention"
)

func (m Model) buildMentionEntries() []mention.MentionEntry {
	assert.That(m.users != nil, "buildMentionEntries: nil users cache")
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
	assert.That(len(entries) >= 3, "buildMentionEntries: special mentions missing")
	return entries
}

func (m *Model) resolvePostUsers(posts []*model.Post) {
	assert.That(m != nil, "resolvePostUsers: nil model")
	assert.That(m.users != nil, "resolvePostUsers: nil users cache")
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
	assert.That(m.usersRequested != nil, "fetchMissingUsers: nil requested set")
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
	assert.That(m.client != nil, "fetchMissingUsers: nil client")
	assert.That(m.scope != nil, "fetchMissingUsers: nil request scope")
	return FetchUsersByIDs(m.reqCtx(), m.client, missing)
}

// usernameMap names every author known so far: real users that have loaded
// (lookups still in flight are nil and skipped) and command pseudo-authors.
func (m Model) usernameMap() map[string]string {
	assert.That(m.users != nil, "usernameMap: nil users cache")
	names := make(map[string]string, len(m.users)+len(m.commandAuthors))
	for id, u := range m.users {
		if u != nil {
			names[id] = u.Username
		}
	}
	maps.Copy(names, m.commandAuthors)
	assert.That(len(names) >= len(m.commandAuthors), "usernameMap: a command author is missing")
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
	assert.That(m.commandAuthors != nil, "commandAuthor: nil command authors")
	name := "/" + slug
	if slug == "" {
		name = "command"
	}
	id := commandResponseUserID + ":" + name
	m.commandAuthors[id] = name
	assert.That(len(id) > len(commandResponseUserID), "commandAuthor: author ID without a name")
	return id
}

// applyMe records the signed-in user, on sign-in and after a profile edit.
// The username decides which @mentions highlight, and the copy in m.users is
// what every post's author line is drawn from; leaving either stale shows
// the old name until the next sign-in.
func (m *Model) applyMe(u *model.User) {
	assert.That(u != nil, "applyMe: nil user")
	assert.That(m.users != nil, "applyMe: nil users cache")
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
	assert.That(m.me == u, "applyMe: user not recorded")
	assert.That(m.users[u.ID] == u, "applyMe: user missing from the cache")
}

// handleUserSearchResults hands user search results to whichever overlay
// asked, if they answer the latest query.
func (m Model) handleUserSearchResults(msg UserSearchResultsMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleUserSearchResults: not signed in")
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
	assert.That(len(filtered) <= len(msg.Users), "handleUserSearchResults: more results than received")
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
	assert.That(m.appState == AppStateRunning, "handleUsersLoaded: not signed in")
	assert.That(m.users != nil, "handleUsersLoaded: nil users cache")
	assert.That(m.usersRequested != nil, "handleUsersLoaded: nil requested set")
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
	assert.That(m.users != nil, "handleUserLoaded: nil users cache")
	assert.That(m.appState == AppStateRunning, "handleUserLoaded: not signed in")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	if m.expiredUserID != "" {
		return m.resumeAfterReLogin(msg.User)
	}
	return m.applyLoadedUser(msg.User)
}

// applyLoadedUser makes u the signed-in user and fetches the users that the
// model still lacks. resumeAfterReLogin calls it directly, not through
// Update, so the call graph has no cycle (Power of 10 rule 1).
func (m Model) applyLoadedUser(u *model.User) (Model, tea.Cmd) {
	assert.That(u != nil, "applyLoadedUser: nil user")
	assert.That(m.appState == AppStateRunning, "applyLoadedUser: not signed in")
	firstLoad := m.me == nil
	m.applyMe(u)
	// Member rows that arrived before the user did could not be
	// matched to them; work their badges out now.
	if firstLoad {
		for channelID, members := range m.channelMembers {
			m.applyMembers(channelID, members)
		}
	}
	assert.That(m.me == u, "applyLoadedUser: user not applied")
	return m, m.fetchMissingUsers()
}

func (m Model) handleProfileUpdated(msg ProfileUpdatedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleProfileUpdated: not signed in")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	var cmds []tea.Cmd
	if msg.User != nil {
		m.applyMe(msg.User)
		assert.That(m.me == msg.User, "handleProfileUpdated: profile not applied")
		cmds = append(cmds, m.setError(errProfileSaved))
	}
	return m, tea.Batch(cmds...)
}

// searchUsers looks up the people matching term, for the palette or the
// DM picker.
func (m Model) searchUsers(term string) (tea.Model, tea.Cmd) {
	assert.That(m.scope != nil, "searchUsers: nil request scope")
	assert.That(m.appState == AppStateRunning, "searchUsers: not signed in")
	assert.That(m.client != nil, "searchUsers: nil client")
	m.userQuery = term
	return m, tea.Batch(SearchUsersCmd(m.reqCtx(), m.client, term))
}

func (m Model) handleAtTrigger(msg input.AtTriggerMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleAtTrigger: not signed in")
	entries := m.buildMentionEntries()
	assert.That(len(entries) >= 3, "handleAtTrigger: special mentions missing")
	m.mention.SetEntries(entries)
	m.mention.Show(msg.Prefix, msg.StartCol)
	return m, nil
}
