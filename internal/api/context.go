package api

import (
	"context"
	"net/http"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

type contextKey string

const (
	userContextKey  contextKey = "user"
	actorContextKey contextKey = "actor"
)

// ContextSetUser stores the authenticated user in the request context.
func ContextSetUser(r *http.Request, user *model.User) *http.Request {
	assert.That(r != nil, "ContextSetUser: nil request")
	assert.That(user != nil, "ContextSetUser: nil user")
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

// ContextGetUser retrieves the authenticated user from the request context.
func ContextGetUser(r *http.Request) *model.User {
	assert.That(r != nil, "ContextGetUser: nil request")
	user, ok := r.Context().Value(userContextKey).(*model.User)
	if !ok {
		return nil
	}
	return user
}

// ContextSetActor records the allowlisted machine actor acting on behalf of
// the request's user (X-On-Behalf-Of). The App reads its ID too, for the
// membership hook's via_actor_id.
func ContextSetActor(r *http.Request, actor *model.User) *http.Request {
	assert.That(r != nil, "ContextSetActor: nil request")
	assert.That(actor != nil, "ContextSetActor: nil actor")
	ctx := context.WithValue(r.Context(), actorContextKey, actor)
	return r.WithContext(app.WithViaActor(ctx, actor.ID))
}

// ContextGetActor returns the machine actor acting on behalf of the request's
// user, or nil when the user acts for themselves.
func ContextGetActor(r *http.Request) *model.User {
	assert.That(r != nil, "ContextGetActor: nil request")
	actor, ok := r.Context().Value(actorContextKey).(*model.User)
	if !ok {
		return nil
	}
	return actor
}
