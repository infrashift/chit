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
			WriteError(w, model.NewInternalError("createPost", err))
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		post, err := a.GetPost(r.Context(), id)
		if err != nil {
			WriteError(w, model.NewNotFoundError("getPost", id))
			return
		}
		WriteJSON(w, http.StatusOK, post)
	}
}

func updatePost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var post model.Post
		if err := json.NewDecoder(r.Body).Decode(&post); err != nil {
			WriteError(w, model.NewBadRequestError("updatePost", "invalid request body"))
			return
		}
		post.ID = id

		updated, err := a.UpdatePost(r.Context(), &post)
		if err != nil {
			WriteError(w, model.NewInternalError("updatePost", err))
			return
		}

		WriteJSON(w, http.StatusOK, updated)
	}
}

func deletePost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := a.DeletePost(id); err != nil {
			WriteError(w, model.NewInternalError("deletePost", err))
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func pinPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := a.PinPost(id); err != nil {
			WriteError(w, model.NewInternalError("pinPost", err))
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func unpinPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := a.UnpinPost(id); err != nil {
			WriteError(w, model.NewInternalError("unpinPost", err))
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getChannelPosts(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channelID := chi.URLParam(r, "id")
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage == 0 {
			perPage = 60
		}

		opts := model.GetPostsOptions{
			Page:    page,
			PerPage: perPage,
		}

		posts, err := a.GetPostsForChannel(r.Context(), channelID, opts)
		if err != nil {
			WriteError(w, model.NewInternalError("getChannelPosts", err))
			return
		}

		WriteJSON(w, http.StatusOK, posts)
	}
}

func getPinnedPosts(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channelID := chi.URLParam(r, "id")
		posts, err := a.GetPinnedPosts(r.Context(), channelID)
		if err != nil {
			WriteError(w, model.NewInternalError("getPinnedPosts", err))
			return
		}
		WriteJSON(w, http.StatusOK, posts)
	}
}
