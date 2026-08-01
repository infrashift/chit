package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

type searchPostsBody struct {
	Terms  string   `json:"terms"`
	TagIDs []string `json:"tag_ids"`
	// From restricts results to a single author, by username. A leading @ is
	// accepted so the value can be pasted straight from a mention.
	From    string `json:"from"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

// searchPostsGlobal searches across every channel the requesting user is a
// member of (no team or channel scope).
func searchPostsGlobal(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)

		var body searchPostsBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, model.NewBadRequestError("searchPostsGlobal", "invalid request body"))
			return
		}
		if body.Terms == "" && len(body.TagIDs) == 0 {
			WriteError(w, model.NewBadRequestError("searchPostsGlobal", "terms or tag_ids required"))
			return
		}
		if body.PerPage == 0 {
			body.PerPage = 60
		}
		body.Page, body.PerPage = clampPagination(body.Page, body.PerPage)

		results, err := a.SearchPostsFrom(r.Context(), "", user.ID, body.Terms, body.From, body.TagIDs, body.Page, body.PerPage)
		if err != nil {
			WriteAppError(w, "searchPostsGlobal", err)
			return
		}

		WriteJSON(w, http.StatusOK, results)
	}
}

func searchPostsInTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		_ = chi.URLParam(r, "id")

		var body searchPostsBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, model.NewBadRequestError("searchPostsInTeam", "invalid request body"))
			return
		}
		if body.Terms == "" && len(body.TagIDs) == 0 {
			WriteError(w, model.NewBadRequestError("searchPostsInTeam", "terms or tag_ids required"))
			return
		}
		if body.PerPage == 0 {
			body.PerPage = 60
		}
		body.Page, body.PerPage = clampPagination(body.Page, body.PerPage)

		results, err := a.SearchPostsFrom(r.Context(), "", user.ID, body.Terms, body.From, body.TagIDs, body.Page, body.PerPage)
		if err != nil {
			WriteAppError(w, "searchPostsInTeam", err)
			return
		}

		WriteJSON(w, http.StatusOK, results)
	}
}

func searchPostsInChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := ContextGetUser(r)
		channelID := chi.URLParam(r, "id")

		var body searchPostsBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, model.NewBadRequestError("searchPostsInChannel", "invalid request body"))
			return
		}
		if body.Terms == "" && len(body.TagIDs) == 0 {
			WriteError(w, model.NewBadRequestError("searchPostsInChannel", "terms or tag_ids required"))
			return
		}
		if body.PerPage == 0 {
			body.PerPage = 60
		}
		body.Page, body.PerPage = clampPagination(body.Page, body.PerPage)

		results, err := a.SearchPostsFrom(r.Context(), channelID, user.ID, body.Terms, body.From, body.TagIDs, body.Page, body.PerPage)
		if err != nil {
			WriteAppError(w, "searchPostsInChannel", err)
			return
		}

		WriteJSON(w, http.StatusOK, results)
	}
}
