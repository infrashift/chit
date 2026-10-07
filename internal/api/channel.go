package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func createChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var channel model.Channel
		if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
			WriteError(w, model.NewBadRequestError("createChannel", "invalid request body"))
			return
		}
		channel.CreatorID = user.ID

		saved, err := a.CreateChannel(r.Context(), &channel)
		if err != nil {
			WriteAppError(w, "createChannel", err)
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		channel, err := a.GetChannel(r.Context(), id)
		if err != nil {
			WriteError(w, model.NewNotFoundError("getChannel", id))
			return
		}
		WriteJSON(w, http.StatusOK, channel)
	}
}

func updateChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")

		existing, err := a.GetChannel(r.Context(), id)
		if err != nil {
			WriteAppError(w, "updateChannel", err)
			return
		}

		var patch model.Channel
		if err = json.NewDecoder(r.Body).Decode(&patch); err != nil {
			WriteError(w, model.NewBadRequestError("updateChannel", "invalid request body"))
			return
		}

		if patch.DisplayName != "" {
			existing.DisplayName = patch.DisplayName
		}
		if patch.Header != "" {
			existing.Header = patch.Header
		}
		if patch.Purpose != "" {
			existing.Purpose = patch.Purpose
		}
		if patch.Name != "" {
			existing.Name = patch.Name
		}

		updated, err := a.UpdateChannel(r.Context(), existing, user.ID)
		if err != nil {
			WriteAppError(w, "updateChannel", err)
			return
		}

		WriteJSON(w, http.StatusOK, updated)
	}
}

func deleteChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		id := chi.URLParam(r, "id")
		if err := a.DeleteChannel(r.Context(), id, user.ID); err != nil {
			WriteAppError(w, "deleteChannel", err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getChannelsForTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teamID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 60)

		channels, err := a.GetChannelsForTeam(r.Context(), teamID, user.ID, page, perPage)
		if err != nil {
			WriteAppError(w, "getChannelsForTeam", err)
			return
		}

		WriteJSON(w, http.StatusOK, channels)
	}
}

func getMyChannels(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teamID := chi.URLParam(r, "id")

		channels, err := a.GetChannelsForUser(r.Context(), user.ID, teamID)
		if err != nil {
			WriteError(w, model.NewInternalError("getMyChannels", err))
			return
		}

		WriteJSON(w, http.StatusOK, channels)
	}
}

func getMyDirectChannels(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)

		channels, err := a.GetDirectChannelsForUser(r.Context(), user.ID)
		if err != nil {
			WriteError(w, model.NewInternalError("getMyDirectChannels", err))
			return
		}

		if channels == nil {
			channels = []*model.Channel{}
		}

		WriteJSON(w, http.StatusOK, channels)
	}
}

func createDirectChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var userIDs []string
		if err := json.NewDecoder(r.Body).Decode(&userIDs); err != nil {
			WriteError(w, model.NewBadRequestError("createDirectChannel", "invalid request body"))
			return
		}
		if len(userIDs) != 2 {
			WriteError(w, model.NewBadRequestError("createDirectChannel", "exactly 2 user IDs required"))
			return
		}

		channel, err := a.CreateDirectChannel(r.Context(), user.ID, userIDs[0], userIDs[1])
		if err != nil {
			WriteAppError(w, "createDirectChannel", err)
			return
		}

		WriteJSON(w, http.StatusCreated, channel)
	}
}

func createGroupChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var userIDs []string
		if err := json.NewDecoder(r.Body).Decode(&userIDs); err != nil {
			WriteError(w, model.NewBadRequestError("createGroupChannel", "invalid request body"))
			return
		}

		channel, err := a.CreateGroupChannel(r.Context(), user.ID, userIDs)
		if err != nil {
			WriteAppError(w, "createGroupChannel", err)
			return
		}

		WriteJSON(w, http.StatusCreated, channel)
	}
}

func addChannelMember(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		var body struct {
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, model.NewBadRequestError("addChannelMember", "invalid request body"))
			return
		}

		member, err := a.AddChannelMember(r.Context(), channelID, body.UserID, user.ID)
		if err != nil {
			WriteAppError(w, "addChannelMember", err)
			return
		}

		WriteJSON(w, http.StatusCreated, member)
	}
}

func removeChannelMember(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		userID := chi.URLParam(r, "user_id")

		if err := a.RemoveChannelMember(r.Context(), channelID, userID, user.ID); err != nil {
			WriteAppError(w, "removeChannelMember", err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getChannelMembers(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 60)

		members, err := a.GetChannelMembers(r.Context(), channelID, user.ID, page, perPage)
		if err != nil {
			WriteAppError(w, "getChannelMembers", err)
			return
		}

		WriteJSON(w, http.StatusOK, members)
	}
}

func viewChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")

		if err := a.UpdateChannelLastViewedAt(r.Context(), channelID, user.ID); err != nil {
			WriteAppError(w, "viewChannel", err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}
