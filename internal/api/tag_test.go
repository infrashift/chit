package api

import (
	"encoding/json"
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tags", strings.NewReader(body))
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tags", strings.NewReader("{bad"))
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/tags", http.NoBody)
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "id", testRootPost)
	r = authedRequest(r, testUser())
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/", http.NoBody)
	r = withChiParams(r, map[string]string{"id": testRootPost, "tag_id": testTagID})
	r = authedRequest(r, testUser())
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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	r = withChiParam(r, "id", testRootPost)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// The batch endpoint exists so opening a channel costs one request rather
// than one per message.
func TestGetTagsForPosts(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getTagsForPosts(a)
	body := `{"post_ids":["` + testRootPost + `"]}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got map[string][]*model.Tag
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, w.Body.String())
	}
}

func TestGetTagsForPosts_EmptyRequest(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"post_ids":[]}`))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	getTagsForPosts(a).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for an empty batch, got %d", w.Code)
	}
}

// The batch is bounded so a client cannot ask for an unbounded set at once.
func TestGetTagsForPosts_RejectsOversizedBatch(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	ids := make([]string, 500)
	for i := range ids {
		ids[i] = testRootPost
	}
	body, _ := json.Marshal(map[string][]string{"post_ids": ids})

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	getTagsForPosts(a).ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for an oversized batch, got %d", w.Code)
	}
}

func TestCreateTag_IsIdempotentAndIgnoresID(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()
	create := func(body string) model.Tag {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
		r = authedRequest(r, testUser())
		w := httptest.NewRecorder()
		createTag(a).ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
		}
		var tag model.Tag
		decodeJSON(t, w.Body, &tag)
		return tag
	}

	forged := "019421a0-0000-7000-8000-0000000000ee"
	first := create(`{"id":"` + forged + `","name":"release"}`)
	if first.ID == forged {
		t.Fatal("a client-chosen tag id was used")
	}
	if again := create(`{"name":"release"}`); again.ID != first.ID {
		t.Fatalf("re-creating a tag gave id %s, want the existing %s", again.ID, first.ID)
	}
}
