package app

import (
	"context"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/model"
	"github.com/infrashift/chit/internal/store"
)

// membershipChannelStore returns a real channel from Get. The shared mention
// mock answers (nil, nil), which no real store does, and which the membership
// path would dereference.
type membershipChannelStore struct {
	*mentionMockChannelStore
	channel *model.Channel
}

// Update returns what it was given. The shared mock answers (nil, nil),
// which no real store does and which the broadcast path dereferences.
func (s *membershipChannelStore) Update(_ context.Context, c *model.Channel) (*model.Channel, error) {
	s.channel = c
	return c, nil
}

func (s *membershipChannelStore) Get(_ context.Context, id string) (*model.Channel, error) {
	if s.channel == nil {
		return nil, errNotFound
	}
	c := *s.channel
	c.ID = id
	return &c, nil
}

// memberChannelID is the channel the membership tests act on. It must be a
// real UUID: UpdateChannel validates the channel before saving it.
const memberChannelID = "019421a0-0000-7000-8000-0000000000c1"

// membershipTestApp reuses the command test harness and seeds a couple of
// users so the membership commands have somebody to resolve.
func membershipTestApp(t *testing.T) *App {
	t.Helper()

	a := newCommandTestApp(t, true)
	ms := a.Store.(*cmdMockStore)
	ms.user.users = map[string]*model.User{
		"alice": {ID: "u-alice", Username: "alice"},
		"bob":   {ID: "u-bob", Username: "bob"},
	}
	// Seed membership so the actor passes requireChannelMember; the rules
	// under test are the ones layered on top of it.
	ms.channel.members = map[string][]*model.ChannelMember{
		memberChannelID: {
			{ChannelID: memberChannelID, UserID: "u-alice"},
			{ChannelID: memberChannelID, UserID: "u-bob"},
		},
	}

	a.Store = &membershipStore{
		cmdMockStore: ms,
		channels: &membershipChannelStore{
			mentionMockChannelStore: ms.channel,
			channel: &model.Channel{
				Type: model.ChannelOpen, TeamID: "t1",
				Name: "general", DisplayName: "General", CreateAt: 1,
			},
		},
		users: &membershipUserStore{
			mentionMockUserStore: &ms.user.mentionMockUserStore,
			byID:                 map[string]*model.User{},
		},
	}
	return a
}

// membershipStore swaps in the stores above; cmdMockStore's fields are
// concretely typed, so they cannot simply be reassigned.
type membershipStore struct {
	*cmdMockStore
	channels store.ChannelStore
	users    *membershipUserStore
}

func (s *membershipStore) Channel() store.ChannelStore { return s.channels }
func (s *membershipStore) User() store.UserStore       { return s.users }

// membershipUserStore resolves users by ID as well as by name. The shared mock
// fails every Get, which would make an admin look like a missing user.
type membershipUserStore struct {
	*mentionMockUserStore
	byID map[string]*model.User
}

func (s *membershipUserStore) Get(_ context.Context, id string) (*model.User, error) {
	u, ok := s.byID[id]
	if !ok {
		return nil, errNotFound
	}
	return u, nil
}

func TestResolveCommandUser(t *testing.T) {
	a := membershipTestApp(t)
	ctx := context.Background()

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
	a := membershipTestApp(t)
	ctx := context.Background()

	tests := []struct {
		name string
		run  func() (string, error)
	}{
		{name: "invite", run: func() (string, error) {
			res, err := a.HandleInvite(ctx, "u-alice", memberChannelID, "nosuchperson")
			if res == nil {
				return "", err
			}
			return res.ResponseText, err
		}},
		{name: "kick", run: func() (string, error) {
			res, err := a.HandleKick(ctx, "u-alice", memberChannelID, "nosuchperson")
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
	a := membershipTestApp(t)
	ctx := context.Background()

	for _, name := range []string{"invite", "kick"} {
		t.Run(name, func(t *testing.T) {
			var text string
			var err error
			if name == "invite" {
				r, e := a.HandleInvite(ctx, "u-alice", memberChannelID, "bob")
				text, err = r.ResponseText, e
			} else {
				r, e := a.HandleKick(ctx, "u-alice", memberChannelID, "bob")
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
		actor     *model.User
		target    string
		wantAllow bool
	}{
		{
			name:      "member removing themselves is leaving",
			actor:     &model.User{ID: "u-alice", Username: "alice"},
			target:    "u-alice",
			wantAllow: true,
		},
		{
			name:      "member removing someone else is refused",
			actor:     &model.User{ID: "u-alice", Username: "alice"},
			target:    "u-bob",
			wantAllow: false,
		},
		{
			name:      "system admin may remove someone else",
			actor:     &model.User{ID: "u-alice", Username: "alice", Roles: "system_admin"},
			target:    "u-bob",
			wantAllow: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := membershipTestApp(t)
			ms := a.Store.(*membershipStore)
			ms.users.byID = map[string]*model.User{tc.actor.ID: tc.actor}

			err := a.RemoveChannelMember(context.Background(), memberChannelID, tc.target, tc.actor.ID)

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
	a := membershipTestApp(t)
	ctx := context.Background()

	t.Run("with no argument reports the current topic", func(t *testing.T) {
		res, err := a.HandleTopic(ctx, "u-alice", memberChannelID, "")
		if err != nil {
			t.Fatalf("HandleTopic: %v", err)
		}
		if res.ResponseText == "" {
			t.Error("no response text")
		}
	})

	t.Run("sets the topic", func(t *testing.T) {
		res, err := a.HandleTopic(ctx, "u-alice", memberChannelID, "  release planning  ")
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
		empty := newCommandTestApp(t, true)
		res, err := empty.HandleTopic(ctx, "u-alice", "missing", "x")
		if err != nil {
			t.Fatalf("expected a text response, got an error: %v", err)
		}
		if res.ResponseText == "" {
			t.Error("no explanation given")
		}
	})
}
