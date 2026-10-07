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
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/commands", http.NoBody)
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

// With no registry (commands disabled) and with an empty one, the list is
// [], not null: the two take different paths to it.
func TestListCommands_NoneAvailable(t *testing.T) {
	for name, reg := range map[string]*command.Registry{
		"no registry":    nil,
		"empty registry": command.NewRegistry([]*command.Command{}),
	} {
		t.Run(name, func(t *testing.T) {
			a, _, cleanup := setupTestApp(t)
			defer cleanup()
			a.CommandRegistry = reg

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/commands", http.NoBody)
			r = authedRequest(r, testUser())
			w := httptest.NewRecorder()
			listCommands(a).ServeHTTP(w, r)

			if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
				t.Fatalf("got %d %s, want 200 []", w.Code, w.Body.String())
			}
		})
	}
}

// Without a command registry (commands disabled), slash text is not a
// command: "/help" is posted like any other message.
func TestCreatePost_WithoutARegistrySlashTextIsAPost(t *testing.T) {
	a, _, cleanup := setupTestApp(t)
	defer cleanup()

	handler := createPost(a)
	body := `{"channel_id":"` + testChannelID + `","content":"/help"}`
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/posts", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = authedRequest(r, testUser())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 when no registry, got %d; body: %s", w.Code, w.Body.String())
	}
}
