package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func serveSearch(t *testing.T, scope searchScope, id, body string, user *model.User) *httptest.ResponseRecorder {
	t.Helper()
	a, _, cleanup := setupTestApp(t)
	t.Cleanup(cleanup)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	if id != "" {
		r = withChiParam(r, "id", id)
	}
	r = authedRequest(r, user)
	w := httptest.NewRecorder()
	searchPosts(a, scope).ServeHTTP(w, r)
	return w
}

func TestSearchPosts_BadRequests(t *testing.T) {
	cases := []struct {
		name  string
		scope searchScope
		id    string
		body  string
	}{
		{"malformed body, everywhere", searchEverywhere, "", "{bad"},
		{"malformed body, team", searchTeam, testTeamID, "{bad"},
		{"malformed body, channel", searchChannel, testChannelID, "{bad"},
		{"neither terms nor tags", searchEverywhere, "", `{}`},
		{"a tag id that is not a uuid", searchEverywhere, "", `{"tag_ids":["bug"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := serveSearch(t, tc.scope, tc.id, tc.body, testUser()); w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d; body: %s", w.Code, w.Body.String())
			}
		})
	}
}

// The team id was read and discarded, so a "team" search was a global one,
// and nobody checked the caller was on the team.
func TestSearchPosts_TeamScopeRequiresMembership(t *testing.T) {
	if w := serveSearch(t, searchTeam, testTeamID, `{"terms":"hello"}`, charlieUser()); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestSearchPosts_EmptyResultIsAnArray(t *testing.T) {
	w := serveSearch(t, searchEverywhere, "", `{"terms":"no such words"}`, testUser())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"order":[]`) {
		t.Fatalf("an empty result must be [], not null: %s", w.Body.String())
	}
}
