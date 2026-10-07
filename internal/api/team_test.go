package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/infrashift/chit/internal/model"
)

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func withChiParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestCreateTeam(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createTeam(a)
	body := `{"name":"newteam","display_name":"New Team","type":"O"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/teams", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var team model.Team
	decodeJSON(t, w.Body, &team)
	if team.CreatorID != testUserID {
		t.Fatalf("expected creator_id=%q, got %q", testUserID, team.CreatorID)
	}
}

func TestCreateTeam_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createTeam(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/teams", strings.NewReader("{bad"))
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// inviteTeamID is an invite-only team whose only member is alice. testUser,
// charlie and dana are not members; dana is a system_admin.
const inviteTeamID = "019421a0-0000-7000-8000-000000000011"

func seedInviteOnlyTeam(ms *mockStore) {
	ms.Teams.Seed(&model.Team{
		ID: inviteTeamID, Name: "secret", DisplayName: "Secret",
		Type: model.TeamInviteOnly, CreatorID: extraUserID, CreateAt: 1000, UpdateAt: 1000,
	})
	ms.Teams.SeedMember(&model.TeamMember{TeamID: inviteTeamID, UserID: extraUserID, Roles: "team_user"})
}

// serveTeamRoute runs handler for a /teams/{id} route as user, with an
// optional JSON body.
func serveTeamRoute(t *testing.T, handler http.HandlerFunc, method, teamID, body string, user *model.User) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequestWithContext(t.Context(), method, "/", http.NoBody)
	} else {
		r = httptest.NewRequestWithContext(t.Context(), method, "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	r = withChiParam(r, "id", teamID)
	r = authedRequest(r, user)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestGetTeam(t *testing.T) {
	cases := []struct {
		name   string
		teamID string
		user   *model.User
		want   int
	}{
		{"open team, non-member", testTeamID, charlieUser(), http.StatusOK},
		{"invite-only team, member", inviteTeamID, aliceUser(), http.StatusOK},
		{"invite-only team, non-member", inviteTeamID, charlieUser(), http.StatusForbidden},
		{"invite-only team, system admin", inviteTeamID, adminUser(), http.StatusOK},
		{"missing team", "019421a0-0000-7000-8000-0000000000ff", testUser(), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, ms, cleanup := setupTestApp(t)
			defer cleanup()
			seedInviteOnlyTeam(ms)

			w := serveTeamRoute(t, getTeam(a), http.MethodGet, tc.teamID, "", tc.user)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d; body: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

// The team update and delete routes took no actor at all, so any
// authenticated caller could rename or delete any team. They now require
// team_admin (granted to the creator) or system_admin.
func TestUpdateTeam(t *testing.T) {
	cases := []struct {
		name string
		user *model.User
		body string
		want int
	}{
		{"team admin", testUser(), `{"display_name":"Updated Team"}`, http.StatusOK},
		{"system admin", adminUser(), `{"display_name":"Updated Team"}`, http.StatusOK},
		{"plain team member", aliceUser(), `{"display_name":"Hijacked"}`, http.StatusForbidden},
		{"non-member", charlieUser(), `{"display_name":"Hijacked"}`, http.StatusForbidden},
		{"rename the slug", testUser(), `{"name":"renamed"}`, http.StatusBadRequest},
		{"invalid display name", testUser(), `{"display_name":""}`, http.StatusBadRequest},
		{"malformed body", testUser(), `{bad`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, ms, cleanup := setupTestApp(t)
			defer cleanup()

			w := serveTeamRoute(t, updateTeam(a), http.MethodPut, testTeamID, tc.body, tc.user)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d; body: %s", tc.want, w.Code, w.Body.String())
			}
			team, _ := ms.Teams.Get(t.Context(), testTeamID)
			if tc.want != http.StatusOK && team.DisplayName != "Engineering" {
				t.Fatalf("a rejected update still changed display_name to %q", team.DisplayName)
			}
		})
	}
}

func TestUpdateTeam_ClearsDescription(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()
	team, _ := ms.Teams.Get(t.Context(), testTeamID)
	team.Description = "old"
	ms.Teams.Seed(team)

	w := serveTeamRoute(t, updateTeam(a), http.MethodPut, testTeamID, `{"description":""}`, testUser())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if team, _ := ms.Teams.Get(t.Context(), testTeamID); team.Description != "" {
		t.Fatal("an explicit empty description should clear it")
	}
}

func TestDeleteTeam(t *testing.T) {
	cases := []struct {
		name string
		user *model.User
		want int
	}{
		{"team admin", testUser(), http.StatusOK},
		{"system admin", adminUser(), http.StatusOK},
		{"plain team member", aliceUser(), http.StatusForbidden},
		{"non-member", charlieUser(), http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, ms, cleanup := setupTestApp(t)
			defer cleanup()

			w := serveTeamRoute(t, deleteTeam(a), http.MethodDelete, testTeamID, "", tc.user)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d; body: %s", tc.want, w.Code, w.Body.String())
			}
			_, err := ms.Teams.Get(t.Context(), testTeamID)
			if deleted := err != nil; deleted != (tc.want == http.StatusOK) {
				t.Fatalf("deleted=%v after a %d response", deleted, w.Code)
			}
		})
	}
}

func TestGetAllTeams(t *testing.T) {
	cases := []struct {
		name       string
		user       *model.User
		wantInvite bool
	}{
		{"non-member sees only open teams", charlieUser(), false},
		{"member sees their invite-only team", aliceUser(), true},
		{"system admin sees every team", adminUser(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, ms, cleanup := setupTestApp(t)
			defer cleanup()
			seedInviteOnlyTeam(ms)

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/teams", http.NoBody)
			r = authedRequest(r, tc.user)
			w := httptest.NewRecorder()
			getAllTeams(a).ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			var teams []model.Team
			decodeJSON(t, w.Body, &teams)
			sawOpen, sawInvite := false, false
			for i := range teams {
				sawOpen = sawOpen || teams[i].ID == testTeamID
				sawInvite = sawInvite || teams[i].ID == inviteTeamID
			}
			if !sawOpen || sawInvite != tc.wantInvite {
				t.Fatalf("open=%v invite=%v, want open=true invite=%v", sawOpen, sawInvite, tc.wantInvite)
			}
		})
	}
}

func TestGetMyTeams(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getMyTeams(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/users/me/teams", http.NoBody)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAddTeamMember(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := addTeamMember(a)
	body := `{"user_id":"` + extraUserID + `"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/teams/"+testTeamID+"/members", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testTeamID)
	// testUser is a member of testTeamID, which is what AddTeamMember requires.
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestRemoveTeamMember(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := removeTeamMember(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
	r = withChiParams(r, map[string]string{"id": testTeamID, "user_id": extraUserID})
	// Removing SOMEBODY ELSE requires system_admin, so the actor is dana.
	r = authedRequest(r, adminUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetTeamMembers(t *testing.T) {
	cases := []struct {
		name string
		user *model.User
		want int
	}{
		{"member", aliceUser(), http.StatusOK},
		{"system admin", adminUser(), http.StatusOK},
		{"non-member", charlieUser(), http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, _, cleanup := setupTestApp(t)
			defer cleanup()

			w := serveTeamRoute(t, getTeamMembers(a), http.MethodGet, testTeamID, "", tc.user)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d; body: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

// Re-adding the team admin must not demote them: SaveMember used to
// overwrite roles on conflict, so any member could strip team_admin from the
// creator by "adding" them again.
func TestAddTeamMember_ReAddKeepsRoles(t *testing.T) {
	a, ms, cleanup := setupTestApp(t)
	defer cleanup()

	w := serveTeamRoute(t, addTeamMember(a), http.MethodPost, testTeamID, `{"user_id":"`+testUserID+`"}`, aliceUser())
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
	m, err := ms.Teams.GetMember(t.Context(), testTeamID, testUserID)
	if err != nil || !m.IsTeamAdmin() {
		t.Fatalf("re-adding the creator dropped team_admin: %+v, %v", m, err)
	}
}
