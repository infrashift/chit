package api

import (
	"net/http"
	"strconv"
	"time"

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
			FileIDs   []string       `json:"file_ids"`
		}
		if !decodeBody(w, r, &body, "createPost") {
			return
		}
		// mentions is computed by the server; a client-supplied list would be
		// stored as-is whenever the content mentions nobody. on_behalf_of is
		// the server's record of who posted for whom.
		delete(body.Props, "mentions")
		delete(body.Props, model.PropOnBehalfOf)

		// With X-On-Behalf-Of, user is the person and actor the client.
		saved, err := a.CreatePostAs(r.Context(), &model.Post{
			ChannelID: body.ChannelID,
			RootID:    body.RootID,
			Content:   body.Content,
			Props:     body.Props,
			FileIDs:   body.FileIDs,
			UserID:    user.ID,
		}, ContextGetActor(r))
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
		writeOK(w)
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
		writeOK(w)
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
		writeOK(w)
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

// streamStatusLimit bounds stream_status events per sender and channel: two a
// second, in bursts of five. A progress event every few hundred milliseconds
// is plenty to show activity, and the limit keeps one client from flooding a
// channel's members.
const (
	streamStatusPerSecond = 2
	streamStatusBurst     = 5
)

func sendStreamStatus(a *app.App) http.HandlerFunc {
	limiters := newUserLimiters(streamStatusPerSecond, streamStatusBurst, time.Now())
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		var body struct {
			Status   string `json:"status"`
			StreamID string `json:"stream_id"`
		}
		if !decodeBody(w, r, &body, "sendStreamStatus") {
			return
		}
		if refused, wait := take(time.Now(), []bucket{{"stream_status", limiters, user.ID + " " + channelID}}); refused != nil {
			setRetryAfter(w, wait)
			WriteError(w, model.NewAppError("sendStreamStatus", "stream_status rate limit exceeded", "", http.StatusTooManyRequests))
			return
		}
		if err := a.SendStreamStatus(r.Context(), channelID, user, ContextGetActor(r), body.Status, body.StreamID); err != nil {
			WriteAppError(w, "sendStreamStatus", err)
			return
		}
		writeOK(w)
	}
}
