package api

import (
	"net/http"
	"strconv"
)

const maxPerPage = 200

// parsePagination reads page/per_page query params, applying defaultPerPage
// when per_page is absent and clamping both values to sane bounds.
func parsePagination(r *http.Request, defaultPerPage int) (page, perPage int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage == 0 {
		perPage = defaultPerPage
	}
	return clampPagination(page, perPage)
}

// clampPagination bounds page to >= 0 and per_page to [1, maxPerPage].
func clampPagination(page, perPage int) (clampedPage, clampedPerPage int) {
	if page < 0 {
		page = 0
	}
	if perPage < 1 {
		perPage = 1
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return page, perPage
}
