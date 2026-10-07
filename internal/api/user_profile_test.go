package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func putMe(t *testing.T, body string, user *model.User) *httptest.ResponseRecorder {
	t.Helper()
	a, _, cleanup := setupTestApp(t)
	t.Cleanup(cleanup)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/users/me", strings.NewReader(body))
	r = authedRequest(r, user)
	w := httptest.NewRecorder()
	updateMe(a).ServeHTTP(w, r)
	return w
}

func TestUpdateMe_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"taken username", `{"username":"alice"}`, http.StatusConflict},
		{"username with a space", `{"username":"test user"}`, http.StatusBadRequest},
		{"empty display name", `{"display_name":""}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := putMe(t, tc.body, testUser()); w.Code != tc.want {
				t.Fatalf("expected %d, got %d; body: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestUpdateMe_LowercasesUsername(t *testing.T) {
	w := putMe(t, `{"username":"NewName"}`, testUser())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var u model.User
	decodeJSON(t, w.Body, &u)
	if u.Username != "newname" {
		t.Fatalf("username = %q, want it lowercased so it can be mentioned", u.Username)
	}
}

// The context user is the pointer held in the user cache. A rejected update
// used to leave its changes on it, visible to every request until the entry
// expired.
func TestUpdateMe_DoesNotMutateTheContextUser(t *testing.T) {
	user := testUser()
	if w := putMe(t, `{"username":"alice"}`, user); w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
	if user.Username != "testuser" {
		t.Fatalf("the cached user now says %q after a rejected update", user.Username)
	}
}

func TestGetUser_RedactsIdentity(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", extraUserID)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()
	getUser(a).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var u model.User
	decodeJSON(t, w.Body, &u)
	if u.Email != "" || u.KratosID != "" {
		t.Fatalf("another user's email/kratos_id leaked: %+v", u)
	}
}

func TestGetUsersByIDs(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	serve := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
		r = authedRequest(r, testUser())
		w := httptest.NewRecorder()
		getUsersByIDs(a).ServeHTTP(w, r)
		return w
	}

	w := serve(`["` + extraUserID + `","` + thirdUserID + `"]`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var users []model.User
	decodeJSON(t, w.Body, &users)
	if len(users) != 2 || users[0].Email != "" {
		t.Fatalf("got %+v, want 2 sanitized users", users)
	}

	if w := serve(`[` + strings.Repeat(`"x",`, 200) + `"x"]`); w.Code != http.StatusBadRequest {
		t.Fatalf("201 ids: got %d, want 400", w.Code)
	}
}

func TestSearchUsers(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/users?term=ali", http.NoBody)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()
	searchUsers(a).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var users []model.User
	decodeJSON(t, w.Body, &users)
	if len(users) != 1 || users[0].Username != "alice" || users[0].Email != "" {
		t.Fatalf("got %+v, want only alice, sanitized", users)
	}
}
