package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func createTag(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if !decodeBody(w, r, &body, "createTag") {
			return
		}

		// Creating a tag that exists returns the existing one: the store
		// upserts by name, so this is idempotent rather than a 409.
		saved, err := a.CreateTag(r.Context(), &model.Tag{Name: body.Name})
		if err != nil {
			WriteAppError(w, "createTag", err)
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getAllTags(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tags, err := a.GetAllTags(r.Context())
		if err != nil {
			WriteAppError(w, "getAllTags", err)
			return
		}
		WriteJSON(w, http.StatusOK, tags)
	}
}

func addTagToPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		postID := chi.URLParam(r, "id")
		var body struct {
			TagID string `json:"tag_id"`
		}
		if !decodeBody(w, r, &body, "addTagToPost") {
			return
		}

		if err := a.AddTagToPost(r.Context(), postID, body.TagID, user.ID); err != nil {
			WriteAppError(w, "addTagToPost", err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func removeTagFromPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		postID := chi.URLParam(r, "id")
		tagID := chi.URLParam(r, "tag_id")

		if err := a.RemoveTagFromPost(r.Context(), postID, tagID, user.ID); err != nil {
			WriteAppError(w, "removeTagFromPost", err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getTagsForPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		postID := chi.URLParam(r, "id")
		tags, err := a.GetTagsForPost(r.Context(), postID, user.ID)
		if err != nil {
			WriteAppError(w, "getTagsForPost", err)
			return
		}
		WriteJSON(w, http.StatusOK, tags)
	}
}

// getTagsForPosts returns tags for a batch of posts. It exists so opening a
// channel costs one request instead of one per message.
func getTagsForPosts(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)

		var body struct {
			PostIDs []string `json:"post_ids"`
		}
		if !decodeBody(w, r, &body, "getTagsForPosts") {
			return
		}

		// Bounded so a client cannot ask for the tags of an unbounded set in
		// one request; a page of history is well under this.
		const maxPosts = 200
		if len(body.PostIDs) > maxPosts {
			WriteError(w, model.NewBadRequestError("getTagsForPosts",
				fmt.Sprintf("post_ids is limited to %d entries", maxPosts)))
			return
		}

		tags, err := a.GetTagsForPosts(r.Context(), body.PostIDs, user.ID)
		if err != nil {
			WriteAppError(w, "getTagsForPosts", err)
			return
		}

		WriteJSON(w, http.StatusOK, tags)
	}
}
