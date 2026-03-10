package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/app"
	"github.com/infrashift/chit/internal/model"
)

func searchPostsInTeam(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = chi.URLParam(r, "id")

		var body struct {
			Terms   string   `json:"terms"`
			TagIDs  []string `json:"tag_ids"`
			Page    int      `json:"page"`
			PerPage int      `json:"per_page"`
		}
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

		results, err := a.SearchPosts(r.Context(), "", body.Terms, body.TagIDs, body.Page, body.PerPage)
		if err != nil {
			WriteError(w, model.NewInternalError("searchPostsInTeam", err))
			return
		}

		WriteJSON(w, http.StatusOK, results)
	}
}

func searchPostsInChannel(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channelID := chi.URLParam(r, "id")

		var body struct {
			Terms   string   `json:"terms"`
			TagIDs  []string `json:"tag_ids"`
			Page    int      `json:"page"`
			PerPage int      `json:"per_page"`
		}
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

		results, err := a.SearchPosts(r.Context(), channelID, body.Terms, body.TagIDs, body.Page, body.PerPage)
		if err != nil {
			WriteError(w, model.NewInternalError("searchPostsInChannel", err))
			return
		}

		WriteJSON(w, http.StatusOK, results)
	}
}
