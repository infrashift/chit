package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestCreateTag(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createTag(a)
	body := `{"id":"` + model.NewID() + `","name":"urgent"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestCreateTag_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createTag(a)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/tags", strings.NewReader("{bad"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetAllTags(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getAllTags(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAddTagToPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := addTagToPost(a)
	body := `{"tag_id":"` + testTagID + `"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testRootPost)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRemoveTagFromPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := removeTagFromPost(a)
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	r = withChiParams(r, map[string]string{"id": testRootPost, "tag_id": testTagID})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetTagsForPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getTagsForPost(a)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = withChiParam(r, "id", testRootPost)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
