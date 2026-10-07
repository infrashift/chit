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

// searchScope says which path parameter, if any, narrows a search.
type searchScope int

const (
	searchEverywhere searchScope = iota // every channel the caller is in
	searchTeam                          // the caller's channels in team {id}
	searchChannel                       // channel {id}
)

// searchPosts serves the three search routes, which differ only in scope.
func searchPosts(a *app.App, scope searchScope) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body searchPostsBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, model.NewBadRequestError("searchPosts", "invalid request body"))
			return
		}
		if body.Terms == "" && len(body.TagIDs) == 0 {
			WriteError(w, model.NewBadRequestError("searchPosts", "terms or tag_ids required"))
			return
		}
		if body.PerPage == 0 {
			body.PerPage = 60
		}
		body.Page, body.PerPage = clampPagination(body.Page, body.PerPage)

		req := &app.SearchRequest{
			UserID: ContextGetUser(r).ID, Terms: body.Terms, TagIDs: body.TagIDs,
			From: body.From, Page: body.Page, PerPage: body.PerPage,
		}
		switch scope {
		case searchTeam:
			req.TeamID = chi.URLParam(r, "id")
		case searchChannel:
			req.ChannelID = chi.URLParam(r, "id")
		}

		results, err := a.SearchPosts(r.Context(), req)
		if err != nil {
			WriteAppError(w, "searchPosts", err)
			return
		}
		WriteJSON(w, http.StatusOK, results)
	}
}
