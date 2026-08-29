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
	r := httptest.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader(body))
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
	r := httptest.NewRequest(http.MethodPost, "/api/v1/teams", strings.NewReader("{bad"))
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetTeam(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getTeam(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/teams/"+testTeamID, nil)
	r = withChiParam(r, "id", testTeamID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetTeam_NotFound(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getTeam(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/teams/nonexistent", nil)
	r = withChiParam(r, "id", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateTeam(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updateTeam(a)
	body := `{"display_name":"Updated Team"}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/teams/"+testTeamID, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testTeamID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeleteTeam(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := deleteTeam(a)
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/teams/"+testTeamID, nil)
	r = withChiParam(r, "id", testTeamID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetAllTeams(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getAllTeams(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/teams", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetMyTeams(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getMyTeams(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/teams", nil)
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
	r := httptest.NewRequest(http.MethodPost, "/api/v1/teams/"+testTeamID+"/members", strings.NewReader(body))
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
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
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
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getTeamMembers(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/teams/"+testTeamID+"/members", nil)
	r = withChiParam(r, "id", testTeamID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
