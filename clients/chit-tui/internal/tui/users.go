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
	m.resolveDMDisplayNames()
}
