package app

import (
	"context"

	"github.com/infrashift/chit/internal/model"
)

// ProvisionUser creates or retrieves a local user for the given Kratos
// identity. Results are cached (TTL-bounded) so the per-request auth path does
// not hit the database every time.
func (a *App) ProvisionUser(ctx context.Context, kratosID string) (*model.User, error) {
	if a.userCache != nil {
		if user, ok := a.userCache.Get(kratosID); ok {
			return user, nil
		}
	}

	user, err := a.Store.User().GetByKratosID(ctx, kratosID)
	if err == nil {
		a.cacheUser(user)
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

	saved, err := a.Store.User().Save(ctx, user)
	if err != nil {
		// Race condition: another request may have created this user.
		// Try fetching again.
		existing, err2 := a.Store.User().GetByKratosID(ctx, kratosID)
		if err2 == nil {
			a.cacheUser(existing)
			return existing, nil
		}
		return nil, err
	}

	a.cacheUser(saved)
	return saved, nil
}

// CreateUser provisions a user on an administrator's behalf.
//
// ADMIN ONLY, and it did not used to be anything at all: the handler decoded a
// model.User straight off the wire and saved it with no check of any kind. Since
// the body carries `roles` and `oauth_client_id`, any authenticated caller —
// including an identity auto-provisioned on its own first request — could mint a
// system_admin and bind an OAuth2 client to it. That is a self-service path to
// cluster admin, and it sat on the endpoint the docs point at for provisioning
// agents.
//
// The fields stay settable BY AN ADMIN, deliberately. Binding oauth_client_id is
// how a machine actor is created (users.oauth_client_id is what
// ResolveOAuthClient matches), so stripping it would break the documented agent
// flow while closing nothing that requireSystemAdmin has not already closed.
//
// The first admin is seeded out of band by scripts/seed-uat, so requiring one
// here cannot lock an empty deployment out of itself.
func (a *App) CreateUser(ctx context.Context, user *model.User, actorID string) (*model.User, error) {
	if err := a.requireSystemAdmin(ctx, actorID, "App.CreateUser"); err != nil {
		return nil, err
	}

	user.PreSave()

	saved, err := a.Store.User().Save(ctx, user)
	if err != nil {
		return nil, err
	}
	a.cacheUser(saved)
	return saved, nil
}

// oauthCacheKey namespaces OAuth2 client IDs away from Kratos IDs, which
// share the one user cache.
func oauthCacheKey(clientID string) string {
	return "oauth:" + clientID
}

func (a *App) cacheUser(user *model.User) {
	if a.userCache == nil {
		return
	}
	if user.KratosID != "" {
		a.userCache.Set(user.KratosID, user)
	}
	if user.OAuthClientID != "" {
		a.userCache.Set(oauthCacheKey(user.OAuthClientID), user)
	}
}

// invalidateUserCache drops a user's cached auth entries after profile
// changes. A machine actor is cached under both its Kratos ID and its OAuth2
// client ID, so both must go or the client-credentials path serves stale data
// until the TTL expires.
func (a *App) invalidateUserCache(user *model.User) {
	if a.userCache == nil || user == nil {
		return
	}
	if user.KratosID != "" {
		a.userCache.Remove(user.KratosID)
	}
	if user.OAuthClientID != "" {
		a.userCache.Remove(oauthCacheKey(user.OAuthClientID))
	}
}

// ResolveOAuthClient returns the machine actor bound to an OAuth2 client ID.
//
// Unlike ProvisionUser there is deliberately no just-in-time creation: a
// Kratos identity that reaches chitd has already authenticated against Kratos,
// whereas an unrecognised OAuth2 client must be rejected. Binding a client to
// a user is an explicit provisioning step.
func (a *App) ResolveOAuthClient(ctx context.Context, clientID string) (*model.User, error) {
	if clientID == "" {
		return nil, model.NewUnauthorizedError("App.ResolveOAuthClient", "empty client id")
	}
	if a.userCache != nil {
		if user, ok := a.userCache.Get(oauthCacheKey(clientID)); ok {
			return user, nil
		}
	}

	user, err := a.Store.User().GetByOAuthClientID(ctx, clientID)
	if err != nil {
		return nil, model.NewUnauthorizedError("App.ResolveOAuthClient",
			"oauth client is not bound to a user")
	}

	a.cacheUser(user)
	return user, nil
}

// GetUser retrieves a user by ID.
func (a *App) GetUser(ctx context.Context, id string) (*model.User, error) {
	return a.Store.User().Get(ctx, id)
}

// GetUserByUsername retrieves a user by username.
func (a *App) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return a.Store.User().GetByUsername(ctx, username)
}

// UpdateUser updates a user's profile.
func (a *App) UpdateUser(ctx context.Context, user *model.User) (*model.User, error) {
	updated, err := a.Store.User().Update(ctx, user)
	if err != nil {
		return nil, err
	}
	a.invalidateUserCache(updated)
	return updated, nil
}

// SearchUsers searches for users by term.
func (a *App) SearchUsers(ctx context.Context, term string, page, perPage int) ([]*model.User, error) {
	return a.Store.User().Search(ctx, term, page, perPage)
}

// GetUsersByIDs retrieves users by their IDs.
func (a *App) GetUsersByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	return a.Store.User().GetByIDs(ctx, ids)
}
