package api

import (
	"net/http"
	"strconv"
)

const (
	maxPerPage = 200
	// maxOffset bounds page*per_page. Without it a huge page overflowed the
	// multiplication into a negative OFFSET, which Postgres rejects with a
	// 500; and no client pages 100,000 rows deep.
	maxOffset = 100_000
)

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

// clampPagination bounds per_page to [1, maxPerPage] and page to
// [0, maxOffset/per_page].
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
	if page > maxOffset/perPage {
		page = maxOffset / perPage
	}
	return page, perPage
}
