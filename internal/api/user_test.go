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

func TestGetMe(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getMe(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var user model.User
	decodeJSON(t, w.Body, &user)
	if user.ID != testUserID {
		t.Fatalf("expected ID=%q, got %q", testUserID, user.ID)
	}
}

func TestGetMe_Unauthed(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getMe(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	// No user in context
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetUser(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getUser(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+testUserID, nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", testUserID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var user model.User
	decodeJSON(t, w.Body, &user)
	// Email should be sanitized
	if user.Email != "" {
		t.Fatalf("expected email to be sanitized (empty), got %q", user.Email)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getUser(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/nonexistent", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetUserByUsername(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := getUserByUsername(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/users/username/testuser", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("username", "testuser")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var user model.User
	decodeJSON(t, w.Body, &user)
	if user.Email != "" {
		t.Fatalf("expected email to be sanitized, got %q", user.Email)
	}
}

func TestUpdateMe(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updateMe(a)
	body := `{"display_name":"Updated Name"}`
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var user model.User
	decodeJSON(t, w.Body, &user)
	if user.DisplayName != "Updated Name" {
		t.Fatalf("expected DisplayName=%q, got %q", "Updated Name", user.DisplayName)
	}
}

func TestUpdateMe_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := updateMe(a)
	r := httptest.NewRequest(http.MethodPut, "/api/v1/users/me", strings.NewReader("not json"))
	r = authedRequest(r, testUser())

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateUser(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createUser(a)
	body := `{"id":"` + model.NewID() + `","kratos_id":"new-kratos","username":"newuser","display_name":"New","email":"new@test.com","roles":"system_user","create_at":1000,"update_at":1000}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestCreateUser_InvalidBody(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createUser(a)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
