package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

func createChannel(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "createChannel: nil app")
	assert.That(a.Store != nil, "createChannel: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		// Only the fields a client may choose. Decoding straight into
		// model.Channel let a caller set id, create_at or delete_at.
		var body struct {
			TeamID      string `json:"team_id"`
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
			Header      string `json:"header"`
			Purpose     string `json:"purpose"`
		}
		if !decodeBody(w, r, &body, "createChannel") {
			return
		}

		assert.That(user != nil, "createChannel: no authenticated user")
		saved, err := a.CreateChannel(r.Context(), &model.Channel{
			TeamID:      body.TeamID,
			Name:        body.Name,
			DisplayName: body.DisplayName,
			Type:        body.Type,
			Header:      body.Header,
			Purpose:     body.Purpose,
			CreatorID:   user.ID,
		})
		if err != nil {
			WriteAppError(w, "createChannel", err)
			return
		}
		assert.That(saved != nil, "createChannel: app returned nil without an error")
		a.AuditOnBehalfOf(ContextGetActor(r), "channel_created", user.ID, "channel_id", saved.ID)

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getChannel(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getChannel: nil app")
	assert.That(a.Store != nil, "getChannel: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		assert.That(ContextGetUser(r) != nil, "getChannel: no authenticated user")
		channel, err := a.GetChannel(r.Context(), chi.URLParam(r, "id"), ContextGetUser(r).ID)
		if err != nil {
			WriteAppError(w, "getChannel", err)
			return
		}
		assert.That(channel != nil, "getChannel: app returned nil without an error")
		WriteJSON(w, http.StatusOK, channel)
	}
}

func updateChannel(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "updateChannel: nil app")
	assert.That(a.Store != nil, "updateChannel: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name        *string `json:"name"`
			DisplayName *string `json:"display_name"`
			Header      *string `json:"header"`
			Purpose     *string `json:"purpose"`
		}
		if !decodeBody(w, r, &body, "updateChannel") {
			return
		}
		if body.Name != nil {
			WriteError(w, model.NewBadRequestError("updateChannel", "channel name cannot be changed"))
			return
		}

		assert.That(ContextGetUser(r) != nil, "updateChannel: no authenticated user")
		updated, err := a.UpdateChannel(r.Context(), chi.URLParam(r, "id"), app.ChannelPatch{
			DisplayName: body.DisplayName,
			Header:      body.Header,
			Purpose:     body.Purpose,
		}, ContextGetUser(r).ID)
		if err != nil {
			WriteAppError(w, "updateChannel", err)
			return
		}
		assert.That(updated != nil, "updateChannel: app returned nil without an error")

		WriteJSON(w, http.StatusOK, updated)
	}
}

func deleteChannel(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "deleteChannel: nil app")
	assert.That(a.Store != nil, "deleteChannel: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		assert.That(user != nil, "deleteChannel: no authenticated user")
		if err := a.DeleteChannel(r.Context(), id, user.ID); err != nil {
			WriteAppError(w, "deleteChannel", err)
			return
		}
		writeOK(w)
	}
}

func getChannelsForTeam(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getChannelsForTeam: nil app")
	assert.That(a.Store != nil, "getChannelsForTeam: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teamID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 60)

		assert.That(user != nil, "getChannelsForTeam: no authenticated user")
		channels, err := a.GetChannelsForTeam(r.Context(), teamID, user.ID, page, perPage)
		if err != nil {
			WriteAppError(w, "getChannelsForTeam", err)
			return
		}

		writeList(w, channels)
	}
}

func getMyChannels(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getMyChannels: nil app")
	assert.That(a.Store != nil, "getMyChannels: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teamID := chi.URLParam(r, "id")

		assert.That(user != nil, "getMyChannels: no authenticated user")
		channels, err := a.GetChannelsForUser(r.Context(), user.ID, teamID)
		if err != nil {
			WriteAppError(w, "getMyChannels", err)
			return
		}

		writeList(w, channels)
	}
}

// getMyChannelMembers returns the caller's member rows for a team's channels
// in one request, so a client can show every channel's unread and mention
// badges without fetching each channel's full member list.
func getMyChannelMembers(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getMyChannelMembers: nil app")
	assert.That(a.Store != nil, "getMyChannelMembers: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teamID := chi.URLParam(r, "id")

		assert.That(user != nil, "getMyChannelMembers: no authenticated user")
		members, err := a.GetMyChannelMembers(r.Context(), user.ID, teamID)
		if err != nil {
			WriteError(w, model.NewInternalError("getMyChannelMembers", err))
			return
		}
		if members == nil {
			members = []*model.ChannelMember{}
		}

		WriteJSON(w, http.StatusOK, members)
	}
}

func getMyDirectChannels(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getMyDirectChannels: nil app")
	assert.That(a.Store != nil, "getMyDirectChannels: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)

		assert.That(user != nil, "getMyDirectChannels: no authenticated user")
		channels, err := a.GetDirectChannelsForUser(r.Context(), user.ID)
		if err != nil {
			WriteAppError(w, "getMyDirectChannels", err)
			return
		}

		writeList(w, channels)
	}
}

func createDirectChannel(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "createDirectChannel: nil app")
	assert.That(a.Store != nil, "createDirectChannel: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var userIDs []string
		if !decodeBody(w, r, &userIDs, "createDirectChannel") {
			return
		}
		if len(userIDs) != 2 {
			WriteError(w, model.NewBadRequestError("createDirectChannel", "exactly 2 user IDs required"))
			return
		}

		assert.That(user != nil, "createDirectChannel: no authenticated user")
		channel, err := a.CreateDirectChannel(r.Context(), user.ID, userIDs[0], userIDs[1])
		if err != nil {
			WriteAppError(w, "createDirectChannel", err)
			return
		}
		assert.That(channel != nil, "createDirectChannel: app returned nil without an error")

		WriteJSON(w, http.StatusCreated, channel)
	}
}

func createGroupChannel(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "createGroupChannel: nil app")
	assert.That(a.Store != nil, "createGroupChannel: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var userIDs []string
		if !decodeBody(w, r, &userIDs, "createGroupChannel") {
			return
		}

		assert.That(user != nil, "createGroupChannel: no authenticated user")
		channel, err := a.CreateGroupChannel(r.Context(), user.ID, userIDs)
		if err != nil {
			WriteAppError(w, "createGroupChannel", err)
			return
		}
		assert.That(channel != nil, "createGroupChannel: app returned nil without an error")

		WriteJSON(w, http.StatusCreated, channel)
	}
}

func addChannelMember(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "addChannelMember: nil app")
	assert.That(a.Store != nil, "addChannelMember: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		var body struct {
			UserID string `json:"user_id"`
		}
		if !decodeBody(w, r, &body, "addChannelMember") {
			return
		}

		assert.That(user != nil, "addChannelMember: no authenticated user")
		member, err := a.AddChannelMember(r.Context(), channelID, body.UserID, user.ID)
		if err != nil {
			WriteAppError(w, "addChannelMember", err)
			return
		}
		assert.That(member != nil, "addChannelMember: app returned nil without an error")
		a.AuditOnBehalfOf(ContextGetActor(r), "channel_member_added", user.ID,
			"channel_id", channelID, "user_id", member.UserID)

		WriteJSON(w, http.StatusCreated, member)
	}
}

func removeChannelMember(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "removeChannelMember: nil app")
	assert.That(a.Store != nil, "removeChannelMember: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		userID := chi.URLParam(r, "user_id")

		assert.That(user != nil, "removeChannelMember: no authenticated user")
		if err := a.RemoveChannelMember(r.Context(), channelID, userID, user.ID); err != nil {
			WriteAppError(w, "removeChannelMember", err)
			return
		}

		writeOK(w)
	}
}

func getChannelMembers(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "getChannelMembers: nil app")
	assert.That(a.Store != nil, "getChannelMembers: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 60)

		assert.That(user != nil, "getChannelMembers: no authenticated user")
		members, err := a.GetChannelMembers(r.Context(), channelID, user.ID, page, perPage)
		if err != nil {
			WriteAppError(w, "getChannelMembers", err)
			return
		}

		writeList(w, members)
	}
}

func viewChannel(a *app.App) http.HandlerFunc {
	assert.That(a != nil, "viewChannel: nil app")
	assert.That(a.Store != nil, "viewChannel: nil store")
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")

		assert.That(user != nil, "viewChannel: no authenticated user")
		if err := a.UpdateChannelLastViewedAt(r.Context(), channelID, user.ID); err != nil {
			WriteAppError(w, "viewChannel", err)
			return
		}

		writeOK(w)
	}
}
