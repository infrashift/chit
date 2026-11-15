package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// activeChannelID is the channel being read, or "" before one is open.
func (m Model) activeChannelID() string {
	if m.activeChan == nil {
		return ""
	}
	return m.activeChan.ID
}

// tagByName finds a known tag usable in channelID, ignoring case. A #tag in
// a message means the channel's tag of that name; a private tag of the same
// name is not picked by accident.
func (m Model) tagByName(name, channelID string) *model.Tag {
	for _, t := range m.allTags {
		if strings.EqualFold(t.Name, name) && t.Scope == model.TagScopeChannel && t.ChannelID == channelID {
			return t
		}
	}
	return nil
}

// searchTagByName picks the tag a #tag in a search means: the active
// channel's tag of that name, or else the user's private tag of that name.
func (m Model) searchTagByName(name, channelID string) *model.Tag {
	if t := m.tagByName(name, channelID); t != nil {
		return t
	}
	for _, t := range m.allTags {
		if strings.EqualFold(t.Name, name) && t.Private() {
			return t
		}
	}
	return nil
}

// knownTag reports whether allTags already holds a tag with this id.
func (m Model) knownTag(id string) bool {
	for _, t := range m.allTags {
		if t.ID == id {
			return true
		}
	}
	return false
}

// tagsUsableIn returns the tags that can be applied in channelID: its
// channel tags and the user's private tags. The picker offers no others.
func (m Model) tagsUsableIn(channelID string) []*model.Tag {
	out := make([]*model.Tag, 0, len(m.allTags))
	for _, t := range m.allTags {
		if t.UsableIn(channelID) {
			out = append(out, t)
		}
	}
	return out
}

// tagPost applies #tags typed in a message to a post. Each is a channel tag
// of the active channel, created there when it does not exist yet.
func (m Model) tagPost(postID string, names []string) []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(names))
	channelID := m.activeChannelID()
	for _, name := range names {
		if t := m.tagByName(name, channelID); t != nil {
			cmds = append(cmds, AddTagToPostCmd(m.reqCtx(), m.client, postID, t.ID))
		} else {
			cmds = append(cmds, CreateTagAndApplyCmd(m.reqCtx(), m.client, name, postID, channelID, false))
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
