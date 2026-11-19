package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/infrashift/chit/internal/model"
)

// ProvisionUser creates or retrieves a local user for the given Kratos
// identity. Results are cached (TTL-bounded) so the per-request auth path does
// not hit the database every time.
//
// A deactivated user is refused (403), not provisioned again: their row
// still holds the Kratos ID, so a second row could not be saved anyway.
func (a *App) ProvisionUser(ctx context.Context, kratosID string) (*model.User, error) {
	user, _, err := a.provisionUser(ctx, kratosID)
	if errors.Is(err, errUserDeactivated) {
		return nil, model.NewForbiddenError("App.ProvisionUser", "this user is deactivated")
	}
	return user, err
}

// errUserDeactivated is provisionUser's error for a Kratos identity whose
// user is deactivated.
//
// semscope: allow(NASA-P10-06) sentinel error: errors.Is needs one identity
var errUserDeactivated = errors.New("user is deactivated")

// provisionUser is ProvisionUser, and also reports whether it created the
// user. A browser request and an on-behalf-of request (X-On-Behalf-Of:
// kratos:<id>) both come here, so both make the same user.
func (a *App) provisionUser(ctx context.Context, kratosID string) (*model.User, bool, error) {
	if a.userCache != nil {
		if user, ok := a.userCache.Get(kratosID); ok {
			return user, false, nil
		}
	}

	user, err := a.Store.User().GetByKratosID(ctx, kratosID)
	if err == nil {
		a.cacheUser(user)
		return user, false, nil
	}
	if !isNotFound(err) {
		return nil, false, err
	}
	deactivated, err := a.Store.User().KratosIDDeactivated(ctx, kratosID)
	if err != nil {
		return nil, false, err
	}
	if deactivated {
		return nil, false, errUserDeactivated
	}

	username, displayName, email, err := a.FetchKratosIdentity(ctx, kratosID)
	if err != nil {
		return nil, false, err
	}

	// An identity without a username trait gets a derived one (username.go).
	// A taken derived name is not an error: the next candidate is longer.
	candidates := usernameCandidates(username, email, kratosID)
	if displayName == "" {
		displayName = candidates[0]
	}
	for i, candidate := range candidates {
		user := &model.User{
			KratosID:    kratosID,
			Username:    candidate,
			DisplayName: displayName,
			Email:       email,
		}
		saved, err := a.Store.User().Save(ctx, user)
		if err == nil {
			a.cacheUser(saved)
			return saved, true, nil
		}
		// Race condition: another request may have created this user.
		// Try fetching again.
		if existing, err2 := a.Store.User().GetByKratosID(ctx, kratosID); err2 == nil {
			a.cacheUser(existing)
			return existing, false, nil
		}
		var appErr *model.AppError
		if i == len(candidates)-1 || !errors.As(err, &appErr) || appErr.StatusCode != http.StatusConflict {
			return nil, false, err
		}
	}
	return nil, false, errors.New("no username candidate") // unreachable: candidates is never empty
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

// UserPatch is a change to the caller's own profile. A nil field is left
// unchanged.
type UserPatch struct {
	Username    *string
	DisplayName *string
}

// UpdateMe applies patch to userID's own profile.
//
// It edits a fresh copy read from the store. The handler used to mutate the
// *model.User from the request context, which is the very pointer held in the
// user cache: a rejected update (a taken username) stayed visible to every
// request until the entry expired, and concurrent requests for the same user
// raced on the struct.
func (a *App) UpdateMe(ctx context.Context, userID string, patch UserPatch) (*model.User, error) {
	user, err := a.Store.User().Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if patch.Username != nil {
		// Usernames are stored lowercase (PreSave), and mention lookup
		// lowercases what it parses; a mixed-case name could never be
		// mentioned.
		user.Username = strings.ToLower(*patch.Username)
	}
	if patch.DisplayName != nil {
		user.DisplayName = *patch.DisplayName
	}
	if err := user.IsValid(); err != nil {
		return nil, err
	}

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
