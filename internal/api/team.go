package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func createTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		var body struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			Description string `json:"description"`
			Type        string `json:"type"`
		}
		if !decodeBody(w, r, &body, "createTeam") {
			return
		}

		saved, err := a.CreateTeam(r.Context(), &model.Team{
			Name: body.Name, DisplayName: body.DisplayName, Description: body.Description,
			Type: body.Type, CreatorID: user.ID,
		})
		if err != nil {
			WriteAppError(w, "createTeam", err)
			return
		}

		WriteJSON(w, http.StatusCreated, saved)
	}
}

func getTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		team, err := a.GetTeam(r.Context(), chi.URLParam(r, "id"), ContextGetUser(r))
		if err != nil {
			WriteAppError(w, "getTeam", err)
			return
		}
		WriteJSON(w, http.StatusOK, team)
	}
}

func updateTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name        *string `json:"name"`
			DisplayName *string `json:"display_name"`
			Description *string `json:"description"`
			Type        *string `json:"type"`
		}
		if !decodeBody(w, r, &body, "updateTeam") {
			return
		}
		// The name is the team's URL slug; renaming it would break every link
		// to the team, so it is fixed at creation.
		if body.Name != nil {
			WriteError(w, model.NewBadRequestError("updateTeam", "team name cannot be changed"))
			return
		}

		updated, err := a.UpdateTeam(r.Context(), chi.URLParam(r, "id"), app.TeamPatch{
			DisplayName: body.DisplayName,
			Description: body.Description,
			Type:        body.Type,
		}, ContextGetUser(r))
		if err != nil {
			WriteAppError(w, "updateTeam", err)
			return
		}
		WriteJSON(w, http.StatusOK, updated)
	}
}

func deleteTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := a.DeleteTeam(r.Context(), chi.URLParam(r, "id"), ContextGetUser(r)); err != nil {
			WriteAppError(w, "deleteTeam", err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getAllTeams(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, perPage := parsePagination(r, 60)

		teams, err := a.GetAllTeams(r.Context(), ContextGetUser(r), page, perPage)
		if err != nil {
			WriteAppError(w, "getAllTeams", err)
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
			WriteAppError(w, "getMyTeams", err)
			return
		}
		WriteJSON(w, http.StatusOK, teams)
	}
}

func addTeamMember(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := ContextGetUser(r)
		if actor == nil {
			WriteError(w, model.NewUnauthorizedError("addTeamMember", "not authenticated"))
			return
		}
		teamID := chi.URLParam(r, "id")
		var body struct {
			UserID string `json:"user_id"`
		}
		if !decodeBody(w, r, &body, "addTeamMember") {
			return
		}

		member, err := a.AddTeamMember(r.Context(), teamID, body.UserID, actor.ID)
		if err != nil {
			WriteAppError(w, "addTeamMember", err)
			return
		}

		WriteJSON(w, http.StatusCreated, member)
	}
}

func removeTeamMember(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := ContextGetUser(r)
		if actor == nil {
			WriteError(w, model.NewUnauthorizedError("removeTeamMember", "not authenticated"))
			return
		}
		teamID := chi.URLParam(r, "id")
		userID := chi.URLParam(r, "user_id")

		if err := a.RemoveTeamMember(r.Context(), teamID, userID, actor.ID); err != nil {
			WriteAppError(w, "removeTeamMember", err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "OK"})
	}
}

func getTeamMembers(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "id")
		page, perPage := parsePagination(r, 60)

		members, err := a.GetTeamMembers(r.Context(), teamID, ContextGetUser(r), page, perPage)
		if err != nil {
			WriteAppError(w, "getTeamMembers", err)
			return
		}

		WriteJSON(w, http.StatusOK, members)
	}
}
