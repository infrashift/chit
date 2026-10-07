package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

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
			cmds = append(cmds, AddTagToPostCmd(m.reqCtx(), m.client, postID, t.ID))
		} else {
			cmds = append(cmds, CreateTagAndApplyCmd(m.reqCtx(), m.client, name, postID))
		}
	}
	return cmds
}

// handlePostsTagsLoaded applies a page's tags in one redraw.
func (m Model) handlePostsTagsLoaded(msg PostsTagsLoadedMsg) (tea.Model, tea.Cmd) {
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
}

// handlePostTagsLoaded applies one post's tags.
func (m Model) handlePostTagsLoaded(msg PostTagsLoadedMsg) (tea.Model, tea.Cmd) {
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
}
