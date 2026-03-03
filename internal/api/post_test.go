package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestCreatePost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createPost(a)
	body := `{"channel_id":"` + testChannelID + `","content":"test message"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/posts", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var post model.Post
	decodeJSON(t, w.Body, &post)
	if post.UserID != testUserID {
		t.Fatalf("expected user_id=%q, got %q", testUserID, post.UserID)
	}
}

func TestCreatePost_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createPost(a)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/posts", strings.NewReader("{bad"))
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getPost(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", testRootPost)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var post model.Post
	decodeJSON(t, w.Body, &post)
	if post.ID != testRootPost {
		t.Fatalf("expected ID=%q, got %q", testRootPost, post.ID)
	}
}

func TestGetPost_NotFound(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getPost(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdatePost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updatePost(a)
	body := `{"content":"updated content","channel_id":"` + testChannelID + `"}`
	r := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testRootPost)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestDeletePost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := deletePost(a)
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParam(r, "id", testRootPost)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPinPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := pinPost(a)
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r = withChiParam(r, "id", testRootPost)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestUnpinPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := unpinPost(a)
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r = withChiParam(r, "id", testRootPost)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGetChannelPosts(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getChannelPosts(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetPinnedPosts(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getPinnedPosts(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", testChannelID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
