package tui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/tagpicker"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/prompt"
)

// Tag rename and delete from the tag picker, and the tag events of the
// server (ISSUE-022, ADR-0017). Only the owner of a tag changes it.

const (
	subjectRenameTag = "rename-tag:"
	subjectDeleteTag = "delete-tag:"
)

// routeTagManageMsg routes the messages of the tag rename and delete.
func (m Model) routeTagManageMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	assert.That(m.client != nil && m.scope != nil, "routeTagManageMsg: no client or request scope")
	assert.That(m.appState == AppStateRunning, "routeTagManageMsg: not signed in")
	switch msg := msg.(type) {
	case tagpicker.TagRenameRequestMsg:
		return handled(m.askRenameTag(msg))
	case tagpicker.TagDeleteRequestMsg:
		return handled(m.askDeleteTag(msg))
	case prompt.SubmittedMsg:
		return handled(m.handlePromptSubmitted(msg))
	case prompt.CancelledMsg:
		m.tagPicker.Focus()
		return m, nil, true
	case TagRenamedMsg:
		return handled(m.handleTagRenamed(msg))
	case TagDeletedMsg:
		return handled(m.handleTagDeleted(msg))
	}
	return m.routeChannelTagMsg(msg)
}

// tagByID returns the known tag with id, or nil.
func (m Model) tagByID(id string) *model.Tag {
	assert.That(id != "", "tagByID: empty ID")
	assert.That(m.allTags != nil || len(m.allTags) == 0, "tagByID: tag list not made")
	for _, t := range m.allTags {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// ownTag returns the tag with id when the user owns it, or the error that
// tells why not.
func (m Model) ownTag(id, name string) (*model.Tag, error) {
	assert.That(id != "", "ownTag: empty ID")
	t := m.tagByID(id)
	if t == nil || m.me == nil || t.OwnerID != m.me.ID {
		return nil, fmt.Errorf("only the owner can change #%s", name)
	}
	assert.That(t.ID == id, "ownTag: another tag")
	return t, nil
}

// askRenameTag opens the prompt over the tag picker.
func (m Model) askRenameTag(msg tagpicker.TagRenameRequestMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.TagID != "", "askRenameTag: empty tag ID")
	t, err := m.ownTag(msg.TagID, msg.Name)
	if err != nil {
		return m, m.setError(err)
	}
	m.tagPicker.Blur()
	cmd := m.tagPrompt.Open(subjectRenameTag+t.ID, "Rename #"+t.Name, t.Name, "enter saves · esc cancels")
	assert.That(m.tagPrompt.Visible(), "askRenameTag: prompt not open")
	return m, cmd
}

// askDeleteTag asks before a delete, which cannot be undone.
func (m Model) askDeleteTag(msg tagpicker.TagDeleteRequestMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.TagID != "", "askDeleteTag: empty tag ID")
	assert.That(m.appState == AppStateRunning, "askDeleteTag: not signed in")
	t, err := m.ownTag(msg.TagID, msg.Name)
	if err != nil {
		return m, m.setError(err)
	}
	q := fmt.Sprintf("delete #%s and each use of it? Press ctrl+x again to confirm, any other key to cancel", t.Name)
	return m, m.confirm.Ask(subjectDeleteTag+t.ID, q, key.NewBinding(key.WithKeys("ctrl+x")))
}

// handlePromptSubmitted checks the new name, then renames.
func (m Model) handlePromptSubmitted(msg prompt.SubmittedMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.Key != "", "handlePromptSubmitted: prompt without a key")
	id, ok := strings.CutPrefix(msg.Key, subjectRenameTag)
	if !ok {
		return m, nil
	}
	if err := model.ValidTagName(msg.Value); err != nil {
		m.tagPrompt.SetError(err.Error())
		return m, nil
	}
	assert.That(id != "", "handlePromptSubmitted: empty tag ID")
	return m, RenameTagCmd(m.reqCtx(), m.client, id, model.NormalizeTagName(msg.Value))
}

// handleTagRenamed shows a refusal in the prompt, or applies the rename.
func (m Model) handleTagRenamed(msg TagRenamedMsg) (tea.Model, tea.Cmd) {
	assert.That(m.appState == AppStateRunning, "handleTagRenamed: not signed in")
	if msg.Err != nil {
		if m.tagPrompt.Visible() {
			m.tagPrompt.SetError(api.UserMessage(msg.Err))
			return m, nil
		}
		return m, m.setError(msg.Err)
	}
	assert.That(msg.Tag != nil, "handleTagRenamed: no tag and no error")
	m.tagPrompt.Close()
	m.tagPicker.Focus()
	m.replaceTag(msg.Tag)
	return m, nil
}

// handleTagDeleted removes the tag from each place that shows it.
func (m Model) handleTagDeleted(msg TagDeletedMsg) (tea.Model, tea.Cmd) {
	assert.That(msg.TagID != "", "handleTagDeleted: empty tag ID")
	if msg.Err != nil {
		return m, m.setError(msg.Err)
	}
	m.removeTag(msg.TagID)
	assert.That(m.tagByID(msg.TagID) == nil, "handleTagDeleted: tag kept")
	return m, nil
}

// applyTagEvent applies a tag event of the server. It reports false for
// any other event.
func (m *Model) applyTagEvent(evt model.WebSocketEvent) bool {
	assert.That(evt.Event != "", "applyTagEvent: event without a name")
	assert.That(m.appState == AppStateRunning, "applyTagEvent: not signed in")
	switch evt.Event {
	case model.WebSocketEventPostTagsUpdated:
		m.onPostTagsUpdated(evt)
	case model.WebSocketEventTagUpdated:
		var t model.Tag
		if err := reDecode(evt.Data["tag"], &t); err != nil || t.ID == "" {
			slog.Warn("could not decode a tag_updated payload", "error", err)
			return true
		}
		m.replaceTag(&t)
	case model.WebSocketEventTagDeleted:
		if id := dataString(evt.Data, "tag_id"); id != "" {
			m.removeTag(id)
		}
	case model.WebSocketEventChannelTagsUpdated:
		m.onChannelTagsUpdated(evt)
	default:
		return false
	}
	return true
}

// replaceTag puts t in the place of the tag with its ID: in the known tags,
// on the posts, in the picker and on the channel.
func (m *Model) replaceTag(t *model.Tag) {
	assert.That(m.postTags != nil, "replaceTag: nil post tags")
	assert.That(t != nil && t.ID != "", "replaceTag: no tag")
	for i, old := range m.allTags {
		if old.ID == t.ID {
			m.allTags[i] = t
		}
	}
	m.editPostTags(t.ID, t)
	m.tagPicker.ReplaceTag(t)
	for i, old := range m.channelTags {
		if old.ID == t.ID {
			m.channelTags[i] = t
		}
	}
	m.setChannelTags(m.channelTags)
}

// removeTag takes the tag with id out of each place that shows it. The
// server sends no post_tags_updated after a delete.
func (m *Model) removeTag(id string) {
	assert.That(m.postTags != nil, "removeTag: nil post tags")
	assert.That(id != "", "removeTag: empty ID")
	m.allTags = withoutTag(m.allTags, id)
	m.editPostTags(id, nil)
	m.tagPicker.RemoveTag(id)
	m.setChannelTags(withoutTag(m.channelTags, id))
}

// withoutTag returns tags without the tag with id, in a new slice.
func withoutTag(tags []*model.Tag, id string) []*model.Tag {
	assert.That(id != "", "withoutTag: empty ID")
	out := make([]*model.Tag, 0, len(tags))
	for _, t := range tags {
		if t.ID != id {
			out = append(out, t)
		}
	}
	assert.That(len(out) <= len(tags), "withoutTag: more tags than before")
	return out
}

// editPostTags replaces the tag with id on each post by t, or removes it
// when t is nil, and draws the changed posts again.
func (m *Model) editPostTags(id string, t *model.Tag) {
	assert.That(id != "" && (t == nil || t.ID == id), "editPostTags: the new tag has another ID")
	changed := make(map[string][]string)
	for postID, tags := range m.postTags {
		next := make([]*model.Tag, 0, len(tags))
		hit := false
		for _, old := range tags {
			switch {
			case old.ID != id:
				next = append(next, old)
			case t != nil:
				next, hit = append(next, t), true
			default:
				hit = true
			}
		}
		if hit {
			m.postTags[postID] = next
			changed[postID] = tagNames(next)
		}
	}
	if len(changed) > 0 {
		m.viewport.SetPostsTags(changed)
		m.thread.SetPostTags(m.postTags)
	}
	assert.That(len(changed) <= len(m.postTags), "editPostTags: more changes than posts")
}

// tagNames returns the names of tags, in order.
func tagNames(tags []*model.Tag) []string {
	assert.That(len(tags) <= 1024, "tagNames: more tags than one owner can have")
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	assert.That(len(names) == len(tags), "tagNames: a name for each tag")
	return names
}
