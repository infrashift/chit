package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

func createUser(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "createUser: nil app")
	assert.That(a.Store != nil, "createUser: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		actor := ContextGetUser(r)
		if actor == nil {
			WriteError(w, model.NewUnauthorizedError("createUser", "not authenticated"))
			return
		}

		var user model.User
		if !decodeBody(w, r, &user, "createUser") {
			return
		}

		saved, err := a.CreateUser(r.Context(), &user, actor.ID)
		if err != nil {
			WriteAppError(w, "createUser", err)
			return
		}
		assert.That(saved != nil, "createUser: app returned nil without an error")

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if user == nil {
			WriteError(w, model.NewUnauthorizedError("getMe", "not authenticated"))
			return
		}
		WriteJSON(w, http.StatusOK, user)
	}
}

func updateMe(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "updateMe: nil app")
	assert.That(a.Store != nil, "updateMe: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if user == nil {
			WriteError(w, model.NewUnauthorizedError("updateMe", "not authenticated"))
			return
		}

		var body struct {
			Username    *string `json:"username"`
			DisplayName *string `json:"display_name"`
		}
		if !decodeBody(w, r, &body, "updateMe") {
			return
		}

		updated, err := a.UpdateMe(r.Context(), user.ID, app.UserPatch{
			Username:    body.Username,
			DisplayName: body.DisplayName,
		})
		if err != nil {
			WriteAppError(w, "updateMe", err)
			return
		}
		assert.That(updated != nil, "updateMe: app returned nil without an error")

		WriteJSON(w, http.StatusOK, updated)
	}
}

func getUser(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getUser: nil app")
	assert.That(a.Store != nil, "getUser: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		user, err := a.GetUser(r.Context(), id)
		if err != nil {
			WriteAppError(w, "getUser", err)
			return
		}
		assert.That(user != nil, "getUser: app returned nil without an error")
		user.Sanitize()
		WriteJSON(w, http.StatusOK, user)
	}
}

func getUserByUsername(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getUserByUsername: nil app")
	assert.That(a.Store != nil, "getUserByUsername: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		user, err := a.GetUserByUsername(r.Context(), username)
		if err != nil {
			WriteAppError(w, "getUserByUsername", err)
			return
		}
		assert.That(user != nil, "getUserByUsername: app returned nil without an error")
		user.Sanitize()
		WriteJSON(w, http.StatusOK, user)
	}
}

func searchUsers(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "searchUsers: nil app")
	assert.That(a.Store != nil, "searchUsers: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		term := r.URL.Query().Get("term")
		page, perPage := parsePagination(r, 60)

		users, err := a.SearchUsers(r.Context(), term, page, perPage)
		if err != nil {
			WriteAppError(w, "searchUsers", err)
			return
		}

		for _, u := range users {
			u.Sanitize()
		}

		writeList(w, users)
	}
}

func getUsersByIDs(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getUsersByIDs: nil app")
	assert.That(a.Store != nil, "getUsersByIDs: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		if !decodeBody(w, r, &ids, "getUsersByIDs") {
			return
		}
		const maxIDs = 200
		if len(ids) > maxIDs {
			WriteError(w, model.NewBadRequestError("getUsersByIDs", fmt.Sprintf("at most %d ids", maxIDs)))
			return
		}

		users, err := a.GetUsersByIDs(r.Context(), ids)
		if err != nil {
			WriteAppError(w, "getUsersByIDs", err)
			return
		}

		for _, u := range users {
			u.Sanitize()
		}

		writeList(w, users)
	}
}
