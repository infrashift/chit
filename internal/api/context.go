package api

import (
	"context"
	"net/http"

	"github.com/infrashift/chit/internal/model"
)

type contextKey string

const userContextKey contextKey = "user"

// ContextSetUser stores the authenticated user in the request context.
func ContextSetUser(r *http.Request, user *model.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

// ContextGetUser retrieves the authenticated user from the request context.
func ContextGetUser(r *http.Request) *model.User {
	user, _ := r.Context().Value(userContextKey).(*model.User)
	return user
}
