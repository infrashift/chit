package app

import (
	"slices"
	"sort"
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func channelMemberNames(t *testing.T, f *fixture, channelID string) []string {
	t.Helper()
	members, err := f.store.Channels.GetMembers(t.Context(), channelID, 0, 0)
	if err != nil {
		t.Fatalf("GetMembers: %v", err)
	}
	byID := map[string]string{}
	for name, u := range f.users {
		byID[u.ID] = name
	}
	names := make([]string, 0, len(members))
	for _, m := range members {
		names = append(names, byID[m.UserID])
	}
	sort.Strings(names)
	return names
}

func TestCreateChannel_Membership(t *testing.T) {
	cases := []struct {
		typ  string
		want []string
	}{
		{model.ChannelOpen, []string{"alice", "bob", "creator"}},
		{model.ChannelPrivate, []string{"creator"}},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			f := newFixture(t)
			f.joinTeam("creator", "alice", "bob")
			f.user("outsider") // not on the team: never auto-added

			created, err := f.app.CreateChannel(t.Context(), &model.Channel{
				TeamID: f.team.ID, Name: "new-chan", DisplayName: "New", Type: tc.typ,
				CreatorID: f.user("creator").ID,
			})
			if err != nil {
				t.Fatalf("CreateChannel: %v", err)
			}

			if got := channelMemberNames(t, f, created.ID); !slices.Equal(got, tc.want) {
				t.Fatalf("members = %v, want %v", got, tc.want)
			}
		})
	}
}
