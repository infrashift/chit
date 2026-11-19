package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
)

func getThread(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getThread: nil app")
	assert.That(a.Store != nil, "getThread: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		assert.That(user != nil, "getThread: no authenticated user")
		posts, err := a.GetThread(r.Context(), id, user.ID)
		if err != nil {
			WriteAppError(w, "getThread", err)
			return
		}
		assert.That(posts != nil, "getThread: app returned nil without an error")
		WriteJSON(w, http.StatusOK, posts)
	}
}

func getMyThreads(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getMyThreads: nil app")
	assert.That(a.Store != nil, "getMyThreads: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teamID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 25)

		assert.That(user != nil, "getMyThreads: no authenticated user")
		threads, err := a.GetThreadsForUser(r.Context(), user.ID, teamID, page, perPage)
		if err != nil {
			WriteAppError(w, "getMyThreads", err)
			return
		}
		assert.That(threads != nil, "getMyThreads: app returned nil without an error")

		WriteJSON(w, http.StatusOK, threads)
	}
}

// getMyDirectThreads lists the followed threads in direct and group channels,
// which have no team and so never appear in a team's thread list.
func getMyDirectThreads(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getMyDirectThreads: nil app")
	assert.That(a.Store != nil, "getMyDirectThreads: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		page, perPage := parsePagination(r, 25)
		assert.That(ContextGetUser(r) != nil, "getMyDirectThreads: no authenticated user")
		threads, err := a.GetDirectThreadsForUser(r.Context(), ContextGetUser(r).ID, page, perPage)
		if err != nil {
			WriteAppError(w, "getMyDirectThreads", err)
			return
		}
		assert.That(threads != nil, "getMyDirectThreads: app returned nil without an error")
		WriteJSON(w, http.StatusOK, threads)
	}
}

func markThreadAsRead(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "markThreadAsRead: nil app")
	assert.That(a.Store != nil, "markThreadAsRead: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		threadID := chi.URLParam(r, "id")

		assert.That(user != nil, "markThreadAsRead: no authenticated user")
		if err := a.MarkThreadAsRead(r.Context(), threadID, user.ID); err != nil {
			WriteAppError(w, "markThreadAsRead", err)
			return
		}

		writeOK(w)
	}
}

func updateThreadFollowing(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "updateThreadFollowing: nil app")
	assert.That(a.Store != nil, "updateThreadFollowing: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		threadID := chi.URLParam(r, "id")

		var body struct {
			Following bool `json:"following"`
		}
		if !decodeBody(w, r, &body, "updateThreadFollowing") {
			return
		}

		assert.That(user != nil, "updateThreadFollowing: no authenticated user")
		if err := a.UpdateThreadFollowing(r.Context(), threadID, user.ID, body.Following); err != nil {
			WriteAppError(w, "updateThreadFollowing", err)
			return
		}

		writeOK(w)
	}
}
