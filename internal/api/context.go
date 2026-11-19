package api

import (
	"context"
	"net/http"

	"github.com/infrashift/chit/internal/model"
)

type contextKey string

const (
	userContextKey  contextKey = "user"
	actorContextKey contextKey = "actor"
)

// ContextSetUser stores the authenticated user in the request context.
func ContextSetUser(r *http.Request, user *model.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

// ContextGetUser retrieves the authenticated user from the request context.
func ContextGetUser(r *http.Request) *model.User {
	user, ok := r.Context().Value(userContextKey).(*model.User)
	if !ok {
		return nil
	}
	return user
}

// ContextSetActor records the allowlisted machine actor acting on behalf of
// the request's user (X-On-Behalf-Of).
func ContextSetActor(r *http.Request, actor *model.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), actorContextKey, actor))
}

// ContextGetActor returns the machine actor acting on behalf of the request's
// user, or nil when the user acts for themselves.
func ContextGetActor(r *http.Request) *model.User {
	actor, ok := r.Context().Value(actorContextKey).(*model.User)
	if !ok {
		return nil
	}
	return actor
}
