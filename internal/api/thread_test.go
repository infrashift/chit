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
	r := httptest.NewRequest(http.MethodGet, "/", nil)
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
	r := httptest.NewRequest(http.MethodGet, "/", nil)
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
	r := httptest.NewRequest(http.MethodPut, "/", nil)
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
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
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
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("{bad"))
	r = withChiParams(r, map[string]string{"team_id": testTeamID, "id": testRootPost})
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
