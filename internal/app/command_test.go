package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/model"
)

// newCommandFixture is a fixture with a registry of two commands, /help and
// /kick, and "testuser" in the general channel. Keto answers every permission
// check with ketoAllowed.
func newCommandFixture(t *testing.T, ketoAllowed bool) *fixture {
	t.Helper()
	f := newFixture(t)
	f.keto.setAllowed(ketoAllowed)
	f.join(f.channel, "testuser")

	f.app.CommandRegistry = command.NewRegistry([]*command.Command{
		{ID: "help", Slug: "help", Description: "List available commands", Category: "chat"},
		{ID: "kick", Slug: "kick", Description: "Remove a user", Category: "admin"},
	})
	f.app.CommandHandlers = map[string]command.Handler{
		"help": command.HandlerFunc(func(context.Context, string, string, string) (*command.CommandResult, error) {
			return &command.CommandResult{ResponseText: "Available commands: /help, /kick"}, nil
		}),
		"kick": command.HandlerFunc(func(_ context.Context, _, _, args string) (*command.CommandResult, error) {
			return &command.CommandResult{ResponseText: fmt.Sprintf("Kicked %s", args), Triggered: true}, nil
		}),
	}

	al, err := command.NewAuditLogger("-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = al.Close() })
	f.app.AuditLogger = al
	return f
}

// intercept runs content through the slash-command interceptor as testuser.
func (f *fixture) intercept(content string) (*CommandResponse, bool) {
	f.t.Helper()
	resp, handled, err := f.app.InterceptSlashCommand(f.t.Context(), f.user("testuser").ID, f.channel.ID, content)
	if err != nil {
		f.t.Fatalf("InterceptSlashCommand(%q): %v", content, err)
	}
	return resp, handled
}

func TestInterceptSlashCommand(t *testing.T) {
	t.Run("a normal message is not a command", func(t *testing.T) {
		resp, handled := newCommandFixture(t, true).intercept("hello world")
		if handled || resp != nil {
			t.Fatalf("handled=%v resp=%v, want a pass-through", handled, resp)
		}
	})
	t.Run("an unknown command is answered, not posted", func(t *testing.T) {
		resp, handled := newCommandFixture(t, true).intercept("/nonexistent")
		if !handled || resp == nil || resp.Text == "" {
			t.Fatalf("handled=%v resp=%v, want an explanation", handled, resp)
		}
	})
	t.Run("an authorized command runs its handler", func(t *testing.T) {
		resp, handled := newCommandFixture(t, true).intercept("/help")
		if !handled || resp == nil || resp.Text != "Available commands: /help, /kick" {
			t.Fatalf("handled=%v resp=%v", handled, resp)
		}
	})
	t.Run("a denied command is answered with a refusal", func(t *testing.T) {
		resp, handled := newCommandFixture(t, false).intercept("/help")
		if !handled || resp == nil || resp.Text == "" {
			t.Fatalf("handled=%v resp=%v, want a refusal", handled, resp)
		}
	})
}

func TestInterceptSlashCommand_WebhookTriggered(t *testing.T) {
	f := newCommandFixture(t, true)
	ch := make(chan *command.WebhookEvent, 10)
	f.app.WebhookCh = ch

	if _, handled := f.intercept("/kick alice"); !handled {
		t.Fatal("expected handled=true")
	}

	select {
	case evt := <-ch:
		if evt.CommandSlug != "kick" || evt.Args != "alice" || evt.Status != "executed" {
			t.Errorf("webhook event = %+v", evt)
		}
	default:
		t.Fatal("expected webhook event on channel")
	}
}

func channelPostCount(t *testing.T, f *fixture) int {
	t.Helper()
	list, err := f.store.Posts.GetPostsForChannel(t.Context(), f.channel.ID, model.GetPostsOptions{})
	if err != nil {
		t.Fatalf("GetPostsForChannel: %v", err)
	}
	return len(list.Order)
}

func TestCreatePost_SlashCommandNotPersisted(t *testing.T) {
	f := newCommandFixture(t, true)

	result, err := f.app.CreatePost(t.Context(), &model.Post{
		ChannelID: f.channel.ID, UserID: f.user("testuser").ID, Content: "/help",
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	if result.Type != "command_response" {
		t.Errorf("type = %q, want command_response", result.Type)
	}
	if ephemeral, _ := result.Props["ephemeral"].(bool); !ephemeral {
		t.Error("expected props.ephemeral=true")
	}
	if n := channelPostCount(t, f); n != 0 {
		t.Errorf("a command was stored as %d post(s)", n)
	}
}

func TestCreatePost_NormalPostStillPersisted(t *testing.T) {
	f := newCommandFixture(t, true)

	result, err := f.app.CreatePost(t.Context(), &model.Post{
		ChannelID: f.channel.ID, UserID: f.user("testuser").ID, Content: "hello world",
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	if result.Type == "command_response" {
		t.Error("expected normal post, got command_response")
	}
	if n := channelPostCount(t, f); n != 1 {
		t.Errorf("stored %d posts, want 1", n)
	}
}

// A slash command used to run before the channel membership check, so a
// non-member could run commands in any channel by ID: `/topic` with no
// argument read a private channel's header back to them.
func TestCreatePost_SlashCommandRequiresMembership(t *testing.T) {
	f := newCommandFixture(t, true)
	ran := false
	f.app.CommandHandlers["help"] = command.HandlerFunc(func(context.Context, string, string, string) (*command.CommandResult, error) {
		ran = true
		return &command.CommandResult{ResponseText: "help"}, nil
	})

	_, err := f.app.CreatePost(t.Context(), &model.Post{
		ChannelID: f.channel.ID, UserID: f.user("outsider").ID, Content: "/help",
	})

	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
	if ran {
		t.Fatal("the command ran for a non-member")
	}
}
