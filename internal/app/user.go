package app

import (
	"context"

	"github.com/infrashift/chit/internal/model"
)

// ProvisionUser creates or retrieves a local user for the given Kratos identity.
func (a *App) ProvisionUser(ctx context.Context, kratosID string) (*model.User, error) {
	user, err := a.Store.User().GetByKratosID(kratosID)
	if err == nil {
		return user, nil
	}

	username, displayName, email, err := a.FetchKratosIdentity(ctx, kratosID)
	if err != nil {
		return nil, err
	}

	user = &model.User{
		KratosID:    kratosID,
		Username:    username,
		DisplayName: displayName,
		Email:       email,
	}
	user.PreSave()

	saved, err := a.Store.User().Save(user)
	if err != nil {
		// Race condition: another request may have created this user.
		// Try fetching again.
		existing, err2 := a.Store.User().GetByKratosID(kratosID)
		if err2 == nil {
			return existing, nil
		}
		return nil, err
	}

	return saved, nil
}

// GetUser retrieves a user by ID.
func (a *App) GetUser(id string) (*model.User, error) {
	return a.Store.User().Get(id)
}

// GetUserByUsername retrieves a user by username.
func (a *App) GetUserByUsername(username string) (*model.User, error) {
	return a.Store.User().GetByUsername(username)
}

// UpdateUser updates a user's profile.
func (a *App) UpdateUser(user *model.User) (*model.User, error) {
	return a.Store.User().Update(user)
}

// SearchUsers searches for users by term.
func (a *App) SearchUsers(term string, page, perPage int) ([]*model.User, error) {
	return a.Store.User().Search(term, page, perPage)
}

// GetUsersByIDs retrieves users by their IDs.
func (a *App) GetUsersByIDs(ids []string) ([]*model.User, error) {
	return a.Store.User().GetByIDs(ids)
}
