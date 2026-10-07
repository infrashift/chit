//go:build integration

package sqlstore

import (
	"testing"

	"github.com/infrashift/chit/internal/model"
)

func TestTeamStoreIntegration_CRUDAndMembers(t *testing.T) {
	ss := testStore(t)

	user, err := ss.User().Save(t.Context(), newTestUser("teamowner"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}

	team, err := ss.Team().Save(t.Context(), &model.Team{
		Name:        "crud-team",
		DisplayName: "CRUD Team",
		Type:        model.TeamOpen,
		CreatorID:   user.ID,
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := ss.Team().Get(t.Context(), team.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "crud-team" {
		t.Errorf("Name: got %q", got.Name)
	}

	byName, err := ss.Team().GetByName(t.Context(), "crud-team")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if byName.ID != team.ID {
		t.Errorf("GetByName: got %s, want %s", byName.ID, team.ID)
	}

	// Membership
	if _, err = ss.Team().SaveMember(t.Context(), &model.TeamMember{
		TeamID: team.ID,
		UserID: user.ID,
		Roles:  "team_admin",
	}); err != nil {
		t.Fatalf("SaveMember: %v", err)
	}
	member, err := ss.Team().GetMember(t.Context(), team.ID, user.ID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if member.UserID != user.ID {
		t.Errorf("GetMember: got %+v", member)
	}

	teams, err := ss.Team().GetTeamsForUser(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetTeamsForUser: %v", err)
	}
	if len(teams) != 1 || teams[0].ID != team.ID {
		t.Errorf("GetTeamsForUser: got %v", teams)
	}

	// Non-member lookup is a not-found AppError (authz depends on this).
	if _, err := ss.Team().GetMember(t.Context(), team.ID, "00000000-0000-7000-8000-000000000000"); err == nil {
		t.Error("GetMember for non-member: expected error, got nil")
	}

	if err := ss.Team().RemoveMember(t.Context(), team.ID, user.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if _, err := ss.Team().GetMember(t.Context(), team.ID, user.ID); err == nil {
		t.Error("GetMember after remove: expected error, got nil")
	}

	// Soft delete
	if err := ss.Team().Delete(t.Context(), team.ID, model.GetMillis()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := ss.Team().Get(t.Context(), team.ID); err == nil {
		t.Error("Get after soft delete: expected error, got nil")
	}
}

// GetAll with a viewer lists open teams and the viewer's own invite-only
// teams, never someone else's; "" lists everything.
func TestTeamStoreIntegration_GetAllVisibility(t *testing.T) {
	ss := testStore(t)

	owner, err := ss.User().Save(t.Context(), newTestUser("visowner"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}
	outsider, err := ss.User().Save(t.Context(), newTestUser("visoutsider"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}
	open, err := ss.Team().Save(t.Context(), &model.Team{
		Name: "vis-open", DisplayName: "Open", Type: model.TeamOpen, CreatorID: owner.ID,
	})
	if err != nil {
		t.Fatalf("save open team: %v", err)
	}
	invite, err := ss.Team().Save(t.Context(), &model.Team{
		Name: "vis-invite", DisplayName: "Invite", Type: model.TeamInviteOnly, CreatorID: owner.ID,
	})
	if err != nil {
		t.Fatalf("save invite team: %v", err)
	}
	if _, err = ss.Team().SaveMember(t.Context(), &model.TeamMember{TeamID: invite.ID, UserID: owner.ID}); err != nil {
		t.Fatalf("SaveMember: %v", err)
	}

	ids := func(visibleTo string) map[string]bool {
		t.Helper()
		teams, err := ss.Team().GetAll(t.Context(), visibleTo, 0, 100)
		if err != nil {
			t.Fatalf("GetAll(%q): %v", visibleTo, err)
		}
		got := map[string]bool{}
		for _, tm := range teams {
			got[tm.ID] = true
		}
		return got
	}

	if got := ids(outsider.ID); !got[open.ID] || got[invite.ID] {
		t.Errorf("outsider: got %v, want the open team only", got)
	}
	if got := ids(owner.ID); !got[open.ID] || !got[invite.ID] {
		t.Errorf("member: got %v, want both teams", got)
	}
	if got := ids(""); !got[open.ID] || !got[invite.ID] {
		t.Errorf("unfiltered: got %v, want both teams", got)
	}
}

// Re-saving an existing member must keep their roles. The upsert used to
// overwrite them, so any member could demote the team admin by re-adding them.
func TestTeamStoreIntegration_SaveMemberKeepsRoles(t *testing.T) {
	ss := testStore(t)

	user, err := ss.User().Save(t.Context(), newTestUser("roleskeeper"))
	if err != nil {
		t.Fatalf("save user: %v", err)
	}
	team, err := ss.Team().Save(t.Context(), &model.Team{
		Name: "roles-team", DisplayName: "Roles", Type: model.TeamOpen, CreatorID: user.ID,
	})
	if err != nil {
		t.Fatalf("save team: %v", err)
	}
	if _, err = ss.Team().SaveMember(t.Context(), &model.TeamMember{
		TeamID: team.ID, UserID: user.ID, Roles: "team_admin team_user",
	}); err != nil {
		t.Fatalf("SaveMember (admin): %v", err)
	}

	again, err := ss.Team().SaveMember(t.Context(), &model.TeamMember{TeamID: team.ID, UserID: user.ID})
	if err != nil {
		t.Fatalf("SaveMember (re-add): %v", err)
	}
	if !again.IsTeamAdmin() {
		t.Errorf("re-add returned roles %q, want team_admin kept", again.Roles)
	}
	stored, err := ss.Team().GetMember(t.Context(), team.ID, user.ID)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if !stored.IsTeamAdmin() {
		t.Errorf("stored roles %q, want team_admin kept", stored.Roles)
	}
}
