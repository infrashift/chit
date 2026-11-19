package server

import (
	"testing"

	"github.com/infrashift/chit/internal/command"
)

func TestHelpCommand(t *testing.T) {
	reg := command.NewRegistry([]*command.Command{
		{ID: "help", Slug: "help", Description: "List available commands"},
		{ID: "kick", Slug: "kick", Description: "Remove a user"},
	})
	result, err := helpCommand{reg: reg}.Execute(t.Context(), "user-1", "ch-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "- `/help` — List available commands\n- `/kick` — Remove a user\n"
	if result.ResponseText != want {
		t.Errorf("response = %q, want %q", result.ResponseText, want)
	}
	if result.Triggered {
		t.Error("/help should not trigger a webhook")
	}

	empty, err := helpCommand{reg: command.NewRegistry(nil)}.Execute(t.Context(), "user-1", "ch-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if empty.ResponseText != "No commands available." {
		t.Errorf("empty registry response = %q", empty.ResponseText)
	}
}
