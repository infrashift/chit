package app

import "github.com/infrashift/chit/internal/model"

// CreateTag creates a new tag.
func (a *App) CreateTag(tag *model.Tag) (*model.Tag, error) {
	return a.Store.Tag().Save(tag)
}

// GetAllTags retrieves all tags.
func (a *App) GetAllTags() ([]*model.Tag, error) {
	return a.Store.Tag().GetAll()
}

// AddTagToPost associates a tag with a post.
func (a *App) AddTagToPost(messageID, tagID string) error {
	return a.Store.Tag().AddTagToPost(messageID, tagID)
}

// RemoveTagFromPost removes a tag association from a post.
func (a *App) RemoveTagFromPost(messageID, tagID string) error {
	return a.Store.Tag().RemoveTagFromPost(messageID, tagID)
}

// GetTagsForPost retrieves tags associated with a post.
func (a *App) GetTagsForPost(messageID string) ([]*model.Tag, error) {
	return a.Store.Tag().GetTagsForPost(messageID)
}
