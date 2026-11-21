package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
)

// GetTags returns the tags that the caller can see in channelID: the
// caller's private tags and the channel's own tags.
func (c *httpClient) GetTags(ctx context.Context, channelID string) ([]*model.Tag, error) {
	assert.That(ctx != nil, "GetTags: nil context")
	assert.That(channelID != "", "GetTags: empty channel ID")
	var tags []*model.Tag
	err := c.get(ctx, "/tags?channel_id="+url.QueryEscape(channelID), &tags)
	return tags, err
}

// RenameTag gives a tag a new name. Only the owner can rename it.
func (c *httpClient) RenameTag(ctx context.Context, tagID, name string) (*model.Tag, error) {
	assert.That(ctx != nil, "RenameTag: nil context")
	assert.That(tagID != "" && name != "", "RenameTag: empty tag ID or name")
	var tag model.Tag
	if err := c.put(ctx, fmt.Sprintf("/tags/%s", tagID), map[string]string{"name": name}, &tag); err != nil {
		return nil, err
	}
	return &tag, nil
}

// DeleteTag deletes a tag and each use of it. Only the owner can delete it.
func (c *httpClient) DeleteTag(ctx context.Context, tagID string) error {
	assert.That(ctx != nil, "DeleteTag: nil context")
	assert.That(tagID != "", "DeleteTag: empty tag ID")
	return c.del(ctx, fmt.Sprintf("/tags/%s", tagID))
}

// GetChannelTags returns the tags applied to a channel.
func (c *httpClient) GetChannelTags(ctx context.Context, channelID string) ([]*model.Tag, error) {
	assert.That(ctx != nil, "GetChannelTags: nil context")
	assert.That(channelID != "", "GetChannelTags: empty channel ID")
	var tags []*model.Tag
	err := c.get(ctx, fmt.Sprintf("/channels/%s/tags", channelID), &tags)
	return tags, err
}

// AddChannelTag applies a tag to a channel.
func (c *httpClient) AddChannelTag(ctx context.Context, channelID, tagID string) error {
	assert.That(ctx != nil, "AddChannelTag: nil context")
	assert.That(channelID != "" && tagID != "", "AddChannelTag: empty channel or tag ID")
	return c.post(ctx, fmt.Sprintf("/channels/%s/tags", channelID), map[string]string{"tag_id": tagID}, nil)
}

// RemoveChannelTag removes a tag from a channel.
func (c *httpClient) RemoveChannelTag(ctx context.Context, channelID, tagID string) error {
	assert.That(ctx != nil, "RemoveChannelTag: nil context")
	assert.That(channelID != "" && tagID != "", "RemoveChannelTag: empty channel or tag ID")
	return c.del(ctx, fmt.Sprintf("/channels/%s/tags/%s", channelID, tagID))
}

// FindChannelsByTags returns the caller's channels that carry each of
// tagIDs, in teamID when it is set.
func (c *httpClient) FindChannelsByTags(ctx context.Context, tagIDs []string, teamID string) ([]*model.TaggedChannel, error) {
	assert.That(ctx != nil, "FindChannelsByTags: nil context")
	assert.That(len(tagIDs) >= 1 && len(tagIDs) <= model.MaxSearchTags, "FindChannelsByTags: 1 to 8 tags")
	body := map[string]any{"tag_ids": tagIDs}
	if teamID != "" {
		body["team_id"] = teamID
	}
	var out []*model.TaggedChannel
	err := c.post(ctx, "/tags/channels", body, &out)
	return out, err
}
