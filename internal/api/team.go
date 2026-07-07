package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func createTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var team model.Team
		if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
			WriteError(w, model.NewBadRequestError("createTeam", "invalid request body"))
			return
		}
		team.CreatorID = user.ID

		saved, err := a.CreateTeam(r.Context(), &team)
		if err != nil {
			WriteError(w, model.NewInternalError("createTeam", err))
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		team, err := a.GetTeam(r.Context(), id)
		if err != nil {
			WriteError(w, model.NewNotFoundError("getTeam", id))
			return
		}
		WriteJSON(w, http.StatusOK, team)
	}
}

func updateTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		existing, err := a.GetTeam(r.Context(), id)
		if err != nil {
			WriteError(w, model.NewInternalError("updateTeam", err))
			return
		}

		var patch model.Team
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			WriteError(w, model.NewBadRequestError("updateTeam", "invalid request body"))
			return
		}

		if patch.DisplayName != "" {
			existing.DisplayName = patch.DisplayName
		}
		if patch.Description != "" {
			existing.Description = patch.Description
		}
		if patch.Name != "" {
			existing.Name = patch.Name
		}
		if patch.Type != "" {
			existing.Type = patch.Type
		}

		updated, err := a.UpdateTeam(r.Context(), existing)
		if err != nil {
			WriteError(w, model.NewInternalError("updateTeam", err))
			return
		}

		WriteJSON(w, http.StatusOK, updated)
	}
}

func deleteTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := a.DeleteTeam(r.Context(), id); err != nil {
			WriteError(w, model.NewInternalError("deleteTeam", err))
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getAllTeams(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, perPage := parsePagination(r, 60)

		teams, err := a.GetAllTeams(r.Context(), page, perPage)
		if err != nil {
			WriteError(w, model.NewInternalError("getAllTeams", err))
			return
		}

		WriteJSON(w, http.StatusOK, teams)
	}
}

func getMyTeams(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		teams, err := a.GetTeamsForUser(r.Context(), user.ID)
		if err != nil {
			WriteError(w, model.NewInternalError("getMyTeams", err))
			return
		}
		WriteJSON(w, http.StatusOK, teams)
	}
}

func addTeamMember(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "id")
		var body struct {
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, model.NewBadRequestError("addTeamMember", "invalid request body"))
			return
		}

		member, err := a.AddTeamMember(r.Context(), teamID, body.UserID)
		if err != nil {
			WriteError(w, model.NewInternalError("addTeamMember", err))
			return
		}

		WriteJSON(w, http.StatusCreated, member)
	}
}

func removeTeamMember(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "id")
		userID := chi.URLParam(r, "user_id")

		if err := a.RemoveTeamMember(r.Context(), teamID, userID); err != nil {
			WriteError(w, model.NewInternalError("removeTeamMember", err))
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getTeamMembers(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 60)

		members, err := a.GetTeamMembers(r.Context(), teamID, page, perPage)
		if err != nil {
			WriteError(w, model.NewInternalError("getTeamMembers", err))
			return
		}

		WriteJSON(w, http.StatusOK, members)
	}
}
