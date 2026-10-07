package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
)

func getThread(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		posts, err := a.GetThread(r.Context(), id, user.ID)
		if err != nil {
			WriteAppError(w, "getThread", err)
			return
		}
		WriteJSON(w, http.StatusOK, posts)
	}
}

func getMyThreads(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teamID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 25)

		threads, err := a.GetThreadsForUser(r.Context(), user.ID, teamID, page, perPage)
		if err != nil {
			WriteAppError(w, "getMyThreads", err)
			return
		}

		WriteJSON(w, http.StatusOK, threads)
	}
}

func markThreadAsRead(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		threadID := chi.URLParam(r, "id")

		if err := a.MarkThreadAsRead(r.Context(), threadID, user.ID); err != nil {
			WriteAppError(w, "markThreadAsRead", err)
			return
		}

		writeOK(w)
	}
}

func updateThreadFollowing(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		threadID := chi.URLParam(r, "id")

		var body struct {
			Following bool `json:"following"`
		}
		if !decodeBody(w, r, &body, "updateThreadFollowing") {
			return
		}

		if err := a.UpdateThreadFollowing(r.Context(), threadID, user.ID, body.Following); err != nil {
			WriteAppError(w, "updateThreadFollowing", err)
			return
		}

		writeOK(w)
	}
}
