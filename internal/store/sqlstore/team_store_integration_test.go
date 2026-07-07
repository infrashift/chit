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
	if _, err := ss.Team().SaveMember(t.Context(), &model.TeamMember{
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
