package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func (f *fixture) postAs(name string, channel *model.Channel) error {
	f.t.Helper()
	_, err := f.app.CreatePost(f.t.Context(), &model.Post{ChannelID: channel.ID, UserID: f.user(name).ID, Content: "hi"})
	return err
}

func requireForbidden(t *testing.T, err error, what string) {
	t.Helper()
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.StatusCode != http.StatusForbidden {
		t.Fatalf("%s: got %v, want 403", what, err)
	}
}

// Channel membership is the access check, and leaving a team did not touch
// it: a user removed from a team kept reading and posting in its channels.
func TestRemoveTeamMember_LeavesTheTeamsChannels(t *testing.T) {
	f := newFixture(t)
	private := f.privateChannel("secret")
	f.join(f.channel, "bob")
	f.join(private, "bob")

	if err := f.app.RemoveTeamMember(t.Context(), f.team.ID, f.user("bob").ID, f.user("bob").ID); err != nil {
		t.Fatalf("RemoveTeamMember: %v", err)
	}
	for _, c := range []*model.Channel{f.channel, private} {
		requireForbidden(t, f.postAs("bob", c), "posting to "+c.Name+" after leaving the team")
	}
}

func TestDeleteTeam_DeletesItsChannels(t *testing.T) {
	f := newFixture(t)
	f.join(f.channel, "owner")
	f.admin("owner")

	if err := f.app.DeleteTeam(t.Context(), f.team.ID, f.user("owner")); err != nil {
		t.Fatalf("DeleteTeam: %v", err)
	}
	if _, err := f.store.Channels.Get(t.Context(), f.channel.ID); err == nil {
		t.Fatal("the team's channel outlived the team")
	}
	requireForbidden(t, f.postAs("owner", f.channel), "posting to a deleted team's channel")
}

func TestDeleteChannel_EndsAccess(t *testing.T) {
	f := newFixture(t)
	f.join(f.channel, "bob")
	f.admin("root")

	if err := f.app.DeleteChannel(t.Context(), f.channel.ID, f.user("root").ID); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
	requireForbidden(t, f.postAs("bob", f.channel), "posting to a deleted channel")
}
