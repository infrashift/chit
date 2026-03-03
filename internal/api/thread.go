package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func getThread(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		posts, err := a.GetThread(r.Context(), id)
		if err != nil {
			WriteError(w, model.NewNotFoundError("getThread", id))
			return
		}
		WriteJSON(w, http.StatusOK, posts)
	}
}

func getMyThreads(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teamID := chi.URLParam(r, "id")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage == 0 {
			perPage = 25
		}

		threads, err := a.GetThreadsForUser(user.ID, teamID, page, perPage)
		if err != nil {
			WriteError(w, model.NewInternalError("getMyThreads", err))
			return
		}

		WriteJSON(w, http.StatusOK, threads)
	}
}

func markThreadAsRead(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		threadID := chi.URLParam(r, "id")

		if err := a.MarkThreadAsRead(threadID, user.ID); err != nil {
			WriteError(w, model.NewInternalError("markThreadAsRead", err))
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func updateThreadFollowing(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		threadID := chi.URLParam(r, "id")

		var body struct {
			Following bool `json:"following"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, model.NewBadRequestError("updateThreadFollowing", "invalid request body"))
			return
		}

		if err := a.UpdateThreadFollowing(threadID, user.ID, body.Following); err != nil {
			WriteError(w, model.NewInternalError("updateThreadFollowing", err))
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}
