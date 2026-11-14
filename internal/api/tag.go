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
		user := ContextGetUser(r)
		var body struct {
			Name      string `json:"name"`
			Scope     string `json:"scope"`
			ChannelID string `json:"channel_id"`
		}
		if !decodeBody(w, r, &body, "createTag") {
			return
		}

		// Creating a tag that exists in its scope returns the existing one,
		// so this is idempotent rather than a 409.
		saved, err := a.CreateTag(r.Context(), app.TagCreate{
			Name: body.Name, Scope: body.Scope, ChannelID: body.ChannelID,
		}, user.ID, ContextGetActor(r))
		if err != nil {
			WriteAppError(w, "createTag", err)
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

// getAllTags lists the tags the caller can see: their private tags and the
// channel tags of their channels (of one channel, with ?channel_id=).
func getAllTags(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		tags, err := a.GetVisibleTags(r.Context(), user.ID, r.URL.Query().Get("channel_id"))
		if err != nil {
			WriteAppError(w, "getAllTags", err)
			return
		}
		writeList(w, tags)
	}
}

func renameTag(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var body struct {
			Name string `json:"name"`
		}
		if !decodeBody(w, r, &body, "renameTag") {
			return
		}
		tag, err := a.RenameTag(r.Context(), chi.URLParam(r, "id"), body.Name, user.ID)
		if err != nil {
			WriteAppError(w, "renameTag", err)
			return
		}
		WriteJSON(w, http.StatusOK, tag)
	}
}

func deleteTag(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if err := a.DeleteTag(r.Context(), chi.URLParam(r, "id"), user.ID); err != nil {
			WriteAppError(w, "deleteTag", err)
			return
		}
		writeOK(w)
	}
}

// deleteUserTags deletes every tag a user owns; the user or a system_admin.
func deleteUserTags(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if err := a.DeleteTagsOfUser(r.Context(), chi.URLParam(r, "id"), user.ID); err != nil {
			WriteAppError(w, "deleteUserTags", err)
			return
		}
		writeOK(w)
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

		if err := a.AddTagToPost(r.Context(), postID, body.TagID, user.ID, ContextGetActor(r)); err != nil {
			WriteAppError(w, "addTagToPost", err)
			return
		}

		writeOK(w)
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

		writeOK(w)
	}
}

func addTagToChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var body struct {
			TagID string `json:"tag_id"`
		}
		if !decodeBody(w, r, &body, "addTagToChannel") {
			return
		}
		if err := a.AddTagToChannel(r.Context(), chi.URLParam(r, "id"), body.TagID, user.ID, ContextGetActor(r)); err != nil {
			WriteAppError(w, "addTagToChannel", err)
			return
		}
		writeOK(w)
	}
}

func removeTagFromChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		if err := a.RemoveTagFromChannel(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "tag_id"), user.ID); err != nil {
			WriteAppError(w, "removeTagFromChannel", err)
			return
		}
		writeOK(w)
	}
}

func getTagsForChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		tags, err := a.GetTagsForChannel(r.Context(), chi.URLParam(r, "id"), user.ID)
		if err != nil {
			WriteAppError(w, "getTagsForChannel", err)
			return
		}
		writeList(w, tags)
	}
}

// filterChannelsByTags lists the caller's channels that have every one of up
// to 8 tags, on the channel or on one of its posts.
func filterChannelsByTags(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var body struct {
			TagIDs []string `json:"tag_ids"`
			TeamID string   `json:"team_id"`
		}
		if !decodeBody(w, r, &body, "filterChannelsByTags") {
			return
		}
		out, err := a.FilterChannelsByTags(r.Context(), user.ID, body.TeamID, body.TagIDs)
		if err != nil {
			WriteAppError(w, "filterChannelsByTags", err)
			return
		}
		WriteJSON(w, http.StatusOK, out)
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
		writeList(w, tags)
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
