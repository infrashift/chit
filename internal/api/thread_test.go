package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetThread(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getThread(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testRootPost)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetMyThreads(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getMyThreads(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testTeamID)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMarkThreadAsRead(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := markThreadAsRead(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", http.NoBody)
	r = withChiParams(r, map[string]string{"team_id": testTeamID, "id": testRootPost})
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUpdateThreadFollowing(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updateThreadFollowing(a)
	body := `{"following":true}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParams(r, map[string]string{"team_id": testTeamID, "id": testRootPost})
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUpdateThreadFollowing_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updateThreadFollowing(a)
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", strings.NewReader("{bad"))
	r = withChiParams(r, map[string]string{"team_id": testTeamID, "id": testRootPost})
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// The direct-thread inbox and the team-less read/follow routes are served
// through the full router, request validation included.
func TestDirectThreadRoutes(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	router := New(a)
	serve := func(method, path, body string) int {
		r := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-User-Id", testKratosID)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w.Code
	}

	if got := serve(http.MethodGet, "/api/v1/users/me/threads/direct", ""); got != http.StatusOK {
		t.Errorf("GET direct threads = %d, want 200", got)
	}
	if got := serve(http.MethodPut, "/api/v1/users/me/threads/"+testRootPost+"/following", `{"following":true}`); got != http.StatusOK {
		t.Errorf("PUT following = %d, want 200", got)
	}
	if got := serve(http.MethodPut, "/api/v1/users/me/threads/"+testRootPost+"/read", ""); got != http.StatusOK {
		t.Errorf("PUT read = %d, want 200", got)
	}
}
