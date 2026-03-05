package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/command"
)

func TestListCommands(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	// Set up a command registry on the app.
	cmds := []*command.Command{
		{ID: "help", Slug: "help", Description: "List available commands", Category: "chat"},
		{ID: "kick", Slug: "kick", Description: "Remove a user", Category: "admin"},
		{ID: "topic", Slug: "topic", Description: "Set channel topic", Category: "chat"},
	}
	a.CommandRegistry = command.NewRegistry(cmds)

	handler := listCommands(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/commands", nil)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var result []command.Command
	decodeJSON(t, w.Body, &result)
	if len(result) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(result))
	}

	slugs := map[string]bool{}
	for _, c := range result {
		slugs[c.Slug] = true
	}
	for _, slug := range []string{"help", "kick", "topic"} {
		if !slugs[slug] {
			t.Errorf("missing command slug: %s", slug)
		}
	}
}

func TestListCommands_NoRegistry(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	// CommandRegistry is nil by default.
	handler := listCommands(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/commands", nil)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var result []any
	decodeJSON(t, w.Body, &result)
	if len(result) != 0 {
		t.Fatalf("expected empty array, got %d items", len(result))
	}
}

func TestListCommands_EmptyRegistry(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	a.CommandRegistry = command.NewRegistry([]*command.Command{})

	handler := listCommands(a)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/commands", nil)
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result []any
	decodeJSON(t, w.Body, &result)
	if len(result) != 0 {
		t.Fatalf("expected empty array, got %d items", len(result))
	}
}

func TestCreatePost_SlashCommandIntercepted(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	// Without a command registry, slash commands are NOT intercepted.
	// Verify that /help goes through as a normal post.
	handler := createPost(a)
	body := `{"channel_id":"` + testChannelID + `","content":"/help"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/posts", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 when no registry, got %d; body: %s", w.Code, w.Body.String())
	}
}
