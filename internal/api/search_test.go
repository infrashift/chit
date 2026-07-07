package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchPostsInTeam_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := searchPostsInTeam(a)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
	r = withChiParam(r, "id", testTeamID)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearchPostsGlobal_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := searchPostsGlobal(a)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearchPostsGlobal_RequiresTermsOrTags(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := searchPostsGlobal(a)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSearchPostsInChannel_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := searchPostsInChannel(a)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	r = authedRequest(r, testUser())
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
