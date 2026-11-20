package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/infrashift/chit/internal/assert"
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
	assert.That(r != nil, "parsePagination: nil request")
	assert.That(defaultPerPage >= 1 && defaultPerPage <= maxPerPage, "parsePagination: default per page out of range")
	page = int(queryInt(r, "page", strconv.IntSize))
	perPage = int(queryInt(r, "per_page", strconv.IntSize))
	if perPage == 0 {
		perPage = defaultPerPage
	}
	return clampPagination(page, perPage)
}

// queryInt reads the query parameter name as a base-10 integer of bitSize
// bits. A missing or malformed value reads as 0, and an out-of-range value as
// the nearest bound (strconv's own result for ErrRange), so the callers'
// clamping still applies to it.
func queryInt(r *http.Request, name string, bitSize int) int64 {
	assert.That(name != "", "queryInt: empty parameter name")
	assert.That(bitSize > 0 && bitSize <= 64, "queryInt: bit size out of range")
	v, err := strconv.ParseInt(r.URL.Query().Get(name), 10, bitSize)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0
	}
	return v
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
	assert.That(page >= 0, "clampPagination: negative page")
	assert.That(perPage >= 1 && perPage <= maxPerPage, "clampPagination: per page out of range")
	assert.That(page*perPage <= maxOffset, "clampPagination: offset above maxOffset")
	return page, perPage
}
