package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestContextSetGetUser(t *testing.T) {
	user := &model.User{ID: "user-1", Username: "alice"}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = ContextSetUser(r, user)

	got := ContextGetUser(r)
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got != user {
		t.Fatal("expected same user pointer")
	}
	if got.ID != "user-1" {
		t.Fatalf("expected ID=%q, got %q", "user-1", got.ID)
	}
}

func TestContextGetUser_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	got := ContextGetUser(r)
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
