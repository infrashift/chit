package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func createTag(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tag model.Tag
		if err := json.NewDecoder(r.Body).Decode(&tag); err != nil {
			WriteError(w, model.NewBadRequestError("createTag", "invalid request body"))
			return
		}

		saved, err := a.CreateTag(&tag)
		if err != nil {
			WriteError(w, model.NewInternalError("createTag", err))
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getAllTags(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags, err := a.GetAllTags()
		if err != nil {
			WriteError(w, model.NewInternalError("getAllTags", err))
			return
		}
		WriteJSON(w, http.StatusOK, tags)
	}
}

func addTagToPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID := chi.URLParam(r, "id")
		var body struct {
			TagID string `json:"tag_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, model.NewBadRequestError("addTagToPost", "invalid request body"))
			return
		}

		if err := a.AddTagToPost(postID, body.TagID); err != nil {
			WriteError(w, model.NewInternalError("addTagToPost", err))
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func removeTagFromPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID := chi.URLParam(r, "id")
		tagID := chi.URLParam(r, "tag_id")

		if err := a.RemoveTagFromPost(postID, tagID); err != nil {
			WriteError(w, model.NewInternalError("removeTagFromPost", err))
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getTagsForPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID := chi.URLParam(r, "id")
		tags, err := a.GetTagsForPost(postID)
		if err != nil {
			WriteError(w, model.NewInternalError("getTagsForPost", err))
			return
		}
		WriteJSON(w, http.StatusOK, tags)
	}
}
