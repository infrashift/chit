package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func createPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var post model.Post
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			WriteError(w, model.NewBadRequestError("createPost", "invalid request body"))
			return
		}
		post.UserID = user.ID

		saved, err := a.CreatePost(r.Context(), &post)
		if err != nil {
			WriteAppError(w, "createPost", err)
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		post, err := a.GetPost(r.Context(), id, user.ID)
		if err != nil {
			WriteAppError(w, "getPost", err)
			return
		}
		WriteJSON(w, http.StatusOK, post)
	}
}

func updatePost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		var post model.Post
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			WriteError(w, model.NewBadRequestError("updatePost", "invalid request body"))
			return
		}
		post.ID = id

		updated, err := a.UpdatePost(r.Context(), &post, user.ID)
		if err != nil {
			WriteAppError(w, "updatePost", err)
			return
		}

		WriteJSON(w, http.StatusOK, updated)
	}
}

func deletePost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		if err := a.DeletePost(r.Context(), id, user.ID); err != nil {
			WriteAppError(w, "deletePost", err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func pinPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		if err := a.PinPost(r.Context(), id, user.ID); err != nil {
			WriteAppError(w, "pinPost", err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func unpinPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		if err := a.UnpinPost(r.Context(), id, user.ID); err != nil {
			WriteAppError(w, "unpinPost", err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getChannelPosts(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 60)
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)

		opts := model.GetPostsOptions{
			Page:    page,
			PerPage: perPage,
			Since:   since,
		}

		posts, err := a.GetPostsForChannel(r.Context(), channelID, user.ID, opts)
		if err != nil {
			WriteAppError(w, "getChannelPosts", err)
			return
		}

		WriteJSON(w, http.StatusOK, posts)
	}
}

func getPinnedPosts(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		posts, err := a.GetPinnedPosts(r.Context(), channelID, user.ID)
		if err != nil {
			WriteAppError(w, "getPinnedPosts", err)
			return
		}
		WriteJSON(w, http.StatusOK, posts)
	}
}
