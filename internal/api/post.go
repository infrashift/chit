package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

func createPost(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "createPost: nil app")
	assert.That(a.Store != nil, "createPost: nil store")
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
		assert.That(user != nil, "createPost: no authenticated user")
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
		assert.That(saved != nil, "createPost: app returned nil without an error")

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getPost(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getPost: nil app")
	assert.That(a.Store != nil, "getPost: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		assert.That(user != nil, "getPost: no authenticated user")
		post, err := a.GetPost(r.Context(), id, user.ID)
		if err != nil {
			WriteAppError(w, "getPost", err)
			return
		}
		assert.That(post != nil, "getPost: app returned nil without an error")
		WriteJSON(w, http.StatusOK, post)
	}
}

func updatePost(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "updatePost: nil app")
	assert.That(a.Store != nil, "updatePost: nil store")
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

		assert.That(user != nil, "updatePost: no authenticated user")
		updated, err := a.UpdatePost(r.Context(), id, app.PostPatch{Content: body.Content, Props: body.Props}, user.ID)
		if err != nil {
			WriteAppError(w, "updatePost", err)
			return
		}
		assert.That(updated != nil, "updatePost: app returned nil without an error")

		WriteJSON(w, http.StatusOK, updated)
	}
}

func deletePost(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "deletePost: nil app")
	assert.That(a.Store != nil, "deletePost: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		assert.That(user != nil, "deletePost: no authenticated user")
		if err := a.DeletePost(r.Context(), id, user.ID); err != nil {
			WriteAppError(w, "deletePost", err)
			return
		}
		writeOK(w)
	}
}

func pinPost(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "pinPost: nil app")
	assert.That(a.Store != nil, "pinPost: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		assert.That(user != nil, "pinPost: no authenticated user")
		if err := a.PinPost(r.Context(), id, user.ID); err != nil {
			WriteAppError(w, "pinPost", err)
			return
		}
		writeOK(w)
	}
}

func unpinPost(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "unpinPost: nil app")
	assert.That(a.Store != nil, "unpinPost: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		assert.That(user != nil, "unpinPost: no authenticated user")
		if err := a.UnpinPost(r.Context(), id, user.ID); err != nil {
			WriteAppError(w, "unpinPost", err)
			return
		}
		writeOK(w)
	}
}

func getChannelPosts(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getChannelPosts: nil app")
	assert.That(a.Store != nil, "getChannelPosts: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 60)
		since := queryInt(r, "since", 64)

		opts := model.GetPostsOptions{
			Page:    page,
			PerPage: perPage,
			Since:   since,
		}

		assert.That(user != nil, "getChannelPosts: no authenticated user")
		posts, err := a.GetPostsForChannel(r.Context(), channelID, user.ID, opts)
		if err != nil {
			WriteAppError(w, "getChannelPosts", err)
			return
		}
		assert.That(posts != nil, "getChannelPosts: app returned nil without an error")

		WriteJSON(w, http.StatusOK, posts)
	}
}

func getPinnedPosts(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getPinnedPosts: nil app")
	assert.That(a.Store != nil, "getPinnedPosts: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		assert.That(user != nil, "getPinnedPosts: no authenticated user")
		posts, err := a.GetPinnedPosts(r.Context(), channelID, user.ID)
		if err != nil {
			WriteAppError(w, "getPinnedPosts", err)
			return
		}
		assert.That(posts != nil, "getPinnedPosts: app returned nil without an error")
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
	assert.That(a != nil, "sendStreamStatus: nil app")
	assert.That(a.Store != nil, "sendStreamStatus: nil store")
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
		assert.That(user != nil, "sendStreamStatus: no authenticated user")
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
