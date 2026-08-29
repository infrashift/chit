package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func createUser(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := ContextGetUser(r)
		if actor == nil {
			WriteError(w, model.NewUnauthorizedError("createUser", "not authenticated"))
			return
		}

		var user model.User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			WriteError(w, model.NewBadRequestError("createUser", "invalid request body"))
			return
		}

		saved, err := a.CreateUser(r.Context(), &user, actor.ID)
		if err != nil {
			WriteAppError(w, "createUser", err)
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getMe(_ *app.App) http.HandlerFunc {
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
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if user == nil {
			WriteError(w, model.NewUnauthorizedError("updateMe", "not authenticated"))
			return
		}

		var patch model.User
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			WriteError(w, model.NewBadRequestError("updateMe", "invalid request body"))
			return
		}

		if patch.DisplayName != "" {
			user.DisplayName = patch.DisplayName
		}
		if patch.Username != "" {
			user.Username = patch.Username
		}

		updated, err := a.UpdateUser(r.Context(), user)
		if err != nil {
			WriteError(w, model.NewInternalError("updateMe", err))
			return
		}

		WriteJSON(w, http.StatusOK, updated)
	}
}

func getUser(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		user, err := a.GetUser(r.Context(), id)
		if err != nil {
			WriteError(w, model.NewNotFoundError("getUser", id))
			return
		}
		user.Sanitize()
		WriteJSON(w, http.StatusOK, user)
	}
}

func getUserByUsername(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := chi.URLParam(r, "username")
		user, err := a.GetUserByUsername(r.Context(), username)
		if err != nil {
			WriteError(w, model.NewNotFoundError("getUserByUsername", username))
			return
		}
		user.Sanitize()
		WriteJSON(w, http.StatusOK, user)
	}
}

func searchUsers(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		term := r.URL.Query().Get("term")
		page, perPage := parsePagination(r, 60)

		users, err := a.SearchUsers(r.Context(), term, page, perPage)
		if err != nil {
			WriteError(w, model.NewInternalError("searchUsers", err))
			return
		}

		for _, u := range users {
			u.Sanitize()
		}

		WriteJSON(w, http.StatusOK, users)
	}
}

func getUsersByIDs(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
			WriteError(w, model.NewBadRequestError("getUsersByIDs", "invalid request body"))
			return
		}

		users, err := a.GetUsersByIDs(r.Context(), ids)
		if err != nil {
			WriteError(w, model.NewInternalError("getUsersByIDs", err))
			return
		}

		for _, u := range users {
			u.Sanitize()
		}

		WriteJSON(w, http.StatusOK, users)
	}
}
