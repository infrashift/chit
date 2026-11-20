package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/api"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// The requests of the tag management and the channel tags (ISSUE-022).

// TagRenamedMsg carries the tag after a rename.
type TagRenamedMsg struct {
	Tag *model.Tag
	Err error
}

// TagDeletedMsg tells that the tag TagID is gone.
type TagDeletedMsg struct {
	TagID string
	Err   error
}

// ChannelTagsLoadedMsg carries the tags applied to a channel.
type ChannelTagsLoadedMsg struct {
	ChannelID string
	Tags      []*model.Tag
	Err       error
}

// ChannelTagListLoadedMsg carries the tags that the user can see in a
// channel: the channel's own tags and the user's private ones.
type ChannelTagListLoadedMsg struct {
	ChannelID string
	Tags      []*model.Tag
	Err       error
}

// ChannelTagChangedMsg tells that a tag was added to or removed from a
// channel. Tag is the tag that an add created, if any.
type ChannelTagChangedMsg struct {
	ChannelID string
	Tag       *model.Tag
	Err       error
}

// ChannelsFoundMsg carries the channels that carry each of the tags.
type ChannelsFoundMsg struct {
	Results []*model.TaggedChannel
	Err     error
}

func (m TagRenamedMsg) requestError() error           { return m.Err }
func (m TagDeletedMsg) requestError() error           { return m.Err }
func (m ChannelTagsLoadedMsg) requestError() error    { return m.Err }
func (m ChannelTagListLoadedMsg) requestError() error { return m.Err }
func (m ChannelTagChangedMsg) requestError() error    { return m.Err }
func (m ChannelsFoundMsg) requestError() error        { return m.Err }

// RenameTagCmd renames a tag.
func RenameTagCmd(ctx context.Context, client api.ChitClient, tagID, name string) tea.Cmd {
	assert.That(ctx != nil && client != nil, "RenameTagCmd: nil context or client")
	assert.That(tagID != "" && name != "", "RenameTagCmd: empty tag ID or name")
	return func() tea.Msg {
		t, err := client.RenameTag(ctx, tagID, name)
		return TagRenamedMsg{Tag: t, Err: err}
	}
}

// DeleteTagCmd deletes a tag and each use of it.
func DeleteTagCmd(ctx context.Context, client api.ChitClient, tagID string) tea.Cmd {
	assert.That(ctx != nil && client != nil, "DeleteTagCmd: nil context or client")
	assert.That(tagID != "", "DeleteTagCmd: empty tag ID")
	return func() tea.Msg {
		return TagDeletedMsg{TagID: tagID, Err: client.DeleteTag(ctx, tagID)}
	}
}

// FetchChannelTags gets the tags applied to a channel, and the tags that the
// user can see in it.
func FetchChannelTags(ctx context.Context, client api.ChitClient, channelID string) tea.Cmd {
	assert.That(ctx != nil && client != nil, "FetchChannelTags: nil context or client")
	assert.That(channelID != "", "FetchChannelTags: empty channel ID")
	applied := func() tea.Msg {
		tags, err := client.GetChannelTags(ctx, channelID)
		return ChannelTagsLoadedMsg{ChannelID: channelID, Tags: tags, Err: err}
	}
	usable := func() tea.Msg {
		tags, err := client.GetTags(ctx, channelID)
		return ChannelTagListLoadedMsg{ChannelID: channelID, Tags: tags, Err: err}
	}
	return tea.Batch(applied, usable)
}

// AddChannelTagCmd applies the tag name to a channel. A name with no tag
// yet makes a channel tag of that channel first.
func AddChannelTagCmd(ctx context.Context, client api.ChitClient, channelID, name string, known *model.Tag) tea.Cmd {
	assert.That(ctx != nil && client != nil, "AddChannelTagCmd: nil context or client")
	assert.That(channelID != "" && name != "", "AddChannelTagCmd: empty channel or name")
	return func() tea.Msg {
		var created *model.Tag
		tag := known
		if tag == nil {
			if err := model.ValidTagName(name); err != nil {
				return ChannelTagChangedMsg{ChannelID: channelID, Err: fmt.Errorf("tag %q: %w", name, err)}
			}
			t, err := client.CreateTag(ctx, name, model.TagScopeChannel, channelID)
			if err != nil {
				return ChannelTagChangedMsg{ChannelID: channelID, Err: err}
			}
			tag, created = t, t
		}
		err := client.AddChannelTag(ctx, channelID, tag.ID)
		return ChannelTagChangedMsg{ChannelID: channelID, Tag: created, Err: err}
	}
}

// RemoveChannelTagCmd takes a tag off a channel.
func RemoveChannelTagCmd(ctx context.Context, client api.ChitClient, channelID, tagID string) tea.Cmd {
	assert.That(ctx != nil && client != nil, "RemoveChannelTagCmd: nil context or client")
	assert.That(channelID != "" && tagID != "", "RemoveChannelTagCmd: empty channel or tag ID")
	return func() tea.Msg {
		return ChannelTagChangedMsg{ChannelID: channelID, Err: client.RemoveChannelTag(ctx, channelID, tagID)}
	}
}

// FindChannelsCmd finds the user's channels that carry each of tagIDs.
func FindChannelsCmd(ctx context.Context, client api.ChitClient, tagIDs []string) tea.Cmd {
	assert.That(ctx != nil && client != nil, "FindChannelsCmd: nil context or client")
	assert.That(len(tagIDs) >= 1 && len(tagIDs) <= model.MaxSearchTags, "FindChannelsCmd: 1 to 8 tags")
	return func() tea.Msg {
		found, err := client.FindChannelsByTags(ctx, tagIDs, "")
		return ChannelsFoundMsg{Results: found, Err: err}
	}
}
