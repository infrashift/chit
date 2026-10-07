package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func createPost(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		// Only the fields a client may choose. Decoding into model.Post let a
		// caller set id, create_at (backdating, or pinning a post to the top
		// of history), delete_at, is_pinned, or a "system" type.
		var body struct {
			ChannelID string         `json:"channel_id"`
			RootID    string         `json:"root_id"`
			Content   string         `json:"content"`
			Props     map[string]any `json:"props"`
		}
		if !decodeBody(w, r, &body, "createPost") {
			return
		}
		// mentions is computed by the server; a client-supplied list would be
		// stored as-is whenever the content mentions nobody.
		delete(body.Props, "mentions")

		saved, err := a.CreatePost(r.Context(), &model.Post{
			ChannelID: body.ChannelID,
			RootID:    body.RootID,
			Content:   body.Content,
			Props:     body.Props,
			UserID:    user.ID,
		})
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
		var body struct {
			Content *string        `json:"content"`
			Props   map[string]any `json:"props"`
		}
		if !decodeBody(w, r, &body, "updatePost") {
			return
		}

		updated, err := a.UpdatePost(r.Context(), id, app.PostPatch{Content: body.Content, Props: body.Props}, user.ID)
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
