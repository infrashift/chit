package app

import (
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

// newMembershipFixture puts alice and bob in the general channel.
func newMembershipFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.join(f.channel, "alice", "bob")
	return f
}

func TestResolveCommandUser(t *testing.T) {
	a := newMembershipFixture(t).app
	ctx := t.Context()

	tests := []struct {
		name    string
		args    string
		want    string
		wantErr string
	}{
		{name: "plain username", args: "bob", want: "bob"},
		// The client renders mentions with a leading @, so it is natural to
		// type the command that way too.
		{name: "at prefix", args: "@bob", want: "bob"},
		{name: "surrounding whitespace", args: "  bob  ", want: "bob"},
		// Only the first word matters, so trailing text does not become a
		// confusing "no user named" error.
		{name: "extra words ignored", args: "bob please", want: "bob"},
		{name: "empty explains usage", args: "", wantErr: "Usage"},
		{name: "unknown names the input", args: "nosuchperson", wantErr: "nosuchperson"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.resolveCommandUser(ctx, tc.args)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveCommandUser(%q) succeeded", tc.args)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCommandUser(%q): %v", tc.args, err)
			}
			if got.Username != tc.want {
				t.Errorf("Username = %q, want %q", got.Username, tc.want)
			}
		})
	}
}

// A command failure is reported to the invoker as text rather than returned as
// an error: an error surfaces as the generic "Error executing /invite" instead
// of something the user can act on.
func TestMembershipCommandsReportFailureAsText(t *testing.T) {
	f := newMembershipFixture(t)
	a, ctx, alice, channelID := f.app, t.Context(), f.user("alice").ID, f.channel.ID

	tests := []struct {
		name string
		run  func() (string, error)
	}{
		{name: "invite", run: func() (string, error) {
			res, err := a.HandleInvite(ctx, alice, channelID, "nosuchperson")
			if res == nil {
				return "", err
			}
			return res.ResponseText, err
		}},
		{name: "kick", run: func() (string, error) {
			res, err := a.HandleKick(ctx, alice, channelID, "nosuchperson")
			if res == nil {
				return "", err
			}
			return res.ResponseText, err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text, err := tc.run()
			if err != nil {
				t.Fatalf("expected a text response, got an error: %v", err)
			}
			if !strings.Contains(text, "nosuchperson") {
				t.Errorf("ResponseText = %q, want it to name the user", text)
			}
		})
	}
}

// Whatever the outcome, the reply must name the user so it is legible in the
// channel where it appears.
func TestMembershipCommandsNameTheTarget(t *testing.T) {
	f := newMembershipFixture(t)
	a, ctx, alice, channelID := f.app, t.Context(), f.user("alice").ID, f.channel.ID

	for _, name := range []string{"invite", "kick"} {
		t.Run(name, func(t *testing.T) {
			var text string
			var err error
			if name == "invite" {
				r, e := a.HandleInvite(ctx, alice, channelID, "bob")
				text, err = r.ResponseText, e
			} else {
				r, e := a.HandleKick(ctx, alice, channelID, "bob")
				text, err = r.ResponseText, e
			}
			if err != nil {
				t.Fatalf("expected a text response, got an error: %v", err)
			}
			if !strings.Contains(text, "bob") {
				t.Errorf("ResponseText = %q, want it to name the user", text)
			}
		})
	}
}

// /kick is declared admin-only in the CUE registry, but that restriction lives
// in the command layer and is bypassed by a direct REST call. Removing someone
// else is therefore gated here too; removing yourself is leaving and stays
// open to any member.
func TestRemoveChannelMemberRequiresAdminForOthers(t *testing.T) {
	tests := []struct {
		name      string
		admin     bool
		target    string
		wantAllow bool
	}{
		{name: "member removing themselves is leaving", target: "alice", wantAllow: true},
		{name: "member removing someone else is refused", target: "bob", wantAllow: false},
		{name: "system admin may remove someone else", admin: true, target: "bob", wantAllow: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newMembershipFixture(t)
			if tc.admin {
				f.admin("alice")
			}

			err := f.app.RemoveChannelMember(t.Context(), f.channel.ID, f.user(tc.target).ID, f.user("alice").ID)

			if tc.wantAllow && err != nil {
				t.Errorf("removal was refused: %v", err)
			}
			if !tc.wantAllow {
				if err == nil {
					t.Fatal("a non-admin removed another member")
				}
				if !strings.Contains(err.Error(), "admin") {
					t.Errorf("error should explain the admin requirement, got: %v", err)
				}
			}
		})
	}
}

// /topic was declared in the registry with no handler, so setting a channel
// topic was impossible.
func TestHandleTopic(t *testing.T) {
	f := newMembershipFixture(t)
	a, ctx, alice, channelID := f.app, t.Context(), f.user("alice").ID, f.channel.ID

	t.Run("with no argument reports the current topic", func(t *testing.T) {
		res, err := a.HandleTopic(ctx, alice, channelID, "")
		if err != nil {
			t.Fatalf("HandleTopic: %v", err)
		}
		if res.ResponseText == "" {
			t.Error("no response text")
		}
	})

	t.Run("sets the topic", func(t *testing.T) {
		res, err := a.HandleTopic(ctx, alice, channelID, "  release planning  ")
		if err != nil {
			t.Fatalf("HandleTopic: %v", err)
		}
		if !strings.Contains(res.ResponseText, "release planning") {
			t.Errorf("ResponseText = %q, want it to confirm the topic", res.ResponseText)
		}
	})

	// A failure is reported as text, like the other membership commands, so
	// the user sees why rather than a generic execution error.
	t.Run("unknown channel is reported as text", func(t *testing.T) {
		res, err := a.HandleTopic(ctx, alice, model.NewID(), "x")
		if err != nil {
			t.Fatalf("expected a text response, got an error: %v", err)
		}
		if res.ResponseText == "" {
			t.Error("no explanation given")
		}
	})
}
