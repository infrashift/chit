package app

import (
	"context"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/model"
)

// CreateTeam creates a new team and adds the creator as a member.
func (a *App) CreateTeam(ctx context.Context, team *model.Team) (*model.Team, error) {
	assert.That(team != nil, "App.CreateTeam: nil team")
	assert.That(a.Store != nil && a.Hub != nil, "App.CreateTeam: nil store or hub")
	saved, err := a.Store.Team().Save(ctx, team)
	if err != nil {
		return nil, err
	}

	assert.That(saved != nil && saved.ID != "", "App.CreateTeam: saved team has no id")
	member := &model.TeamMember{
		TeamID: saved.ID,
		UserID: saved.CreatorID,
		Roles:  model.TeamRoleAdmin + " " + model.TeamRoleUser,
	}
	if _, err := a.Store.Team().SaveMember(ctx, member); err != nil {
		return nil, err
	}
	a.Hub.NotifyTeamMembershipChanged(saved.CreatorID, saved.ID, true)

	return saved, nil
}

// GetTeam retrieves a team by ID on actor's behalf. Invite-only teams are
// visible only to their members and to system admins.
func (a *App) GetTeam(ctx context.Context, id string, actor *model.User) (*model.Team, error) {
	assert.That(actor != nil, "App.GetTeam: nil actor")
	assert.That(a.Store != nil, "App.GetTeam: nil store")
	team, err := a.Store.Team().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	assert.That(team != nil, "App.GetTeam: nil team without an error")
	if err := a.requireTeamVisible(ctx, team, actor); err != nil {
		return nil, err
	}
	return team, nil
}

// TeamPatch is a partial team update. A nil field is left unchanged; a
// non-nil one is applied even when empty, so a description can be cleared.
// The name is the team's URL slug and cannot be changed.
type TeamPatch struct {
	DisplayName *string
	Description *string
	Type        *string
}

// UpdateTeam applies patch on actor's behalf. Requires team_admin or
// system_admin.
func (a *App) UpdateTeam(ctx context.Context, id string, patch TeamPatch, actor *model.User) (*model.Team, error) {
	assert.That(actor != nil, "App.UpdateTeam: nil actor")
	assert.That(a.Store != nil, "App.UpdateTeam: nil store")
	team, err := a.Store.Team().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	assert.That(team != nil, "App.UpdateTeam: nil team without an error")
	if err := a.requireTeamAdmin(ctx, id, actor, "App.UpdateTeam"); err != nil {
		return nil, err
	}
	if patch.DisplayName != nil {
		team.DisplayName = *patch.DisplayName
	}
	if patch.Description != nil {
		team.Description = *patch.Description
	}
	if patch.Type != nil {
		team.Type = *patch.Type
	}
	if err := team.IsValid(); err != nil {
		return nil, err
	}
	assert.That(team.ID != "", "App.UpdateTeam: team has no id")
	return a.Store.Team().Update(ctx, team)
}

// DeleteTeam soft-deletes a team on actor's behalf. Requires team_admin or
// system_admin.
func (a *App) DeleteTeam(ctx context.Context, id string, actor *model.User) error {
	assert.That(actor != nil, "App.DeleteTeam: nil actor")
	assert.That(a.Store != nil, "App.DeleteTeam: nil store")
	if _, err := a.Store.Team().Get(ctx, id); err != nil {
		return err
	}
	if err := a.requireTeamAdmin(ctx, id, actor, "App.DeleteTeam"); err != nil {
		return err
	}
	now := model.GetMillis()
	assert.That(now > 0, "App.DeleteTeam: clock before the epoch")
	if err := a.Store.Team().Delete(ctx, id, now); err != nil {
		return err
	}
	// A deleted team's channels go with it. Left alone they stayed fully
	// usable by their members, reachable by ID, on a team that no longer
	// existed.
	channelIDs, err := a.Store.Channel().DeleteForTeam(ctx, id, now)
	if err != nil {
		return err
	}
	for _, channelID := range channelIDs {
		assert.That(channelID != "", "App.DeleteTeam: store returned an empty channel id")
		a.publishChannelDeleted(ctx, channelID)
	}
	return nil
}

// GetAllTeams lists the teams actor can see: every team for a system admin,
// otherwise open teams plus the invite-only teams actor belongs to.
func (a *App) GetAllTeams(ctx context.Context, actor *model.User, page, perPage int) ([]*model.Team, error) {
	assert.That(actor != nil && actor.ID != "", "App.GetAllTeams: actor has no id")
	assert.That(page >= 0 && perPage >= 1, "App.GetAllTeams: pagination not clamped")
	assert.That(a.Store != nil, "App.GetAllTeams: nil store")
	visibleTo := actor.ID
	if actor.IsSystemAdmin() {
		visibleTo = ""
	}
	return a.Store.Team().GetAll(ctx, visibleTo, page, perPage)
}

// GetTeamsForUser retrieves teams that a user is a member of.
func (a *App) GetTeamsForUser(ctx context.Context, userID string) ([]*model.Team, error) {
	assert.That(a.Store != nil, "App.GetTeamsForUser: nil store")
	assert.That(userID != "", "App.GetTeamsForUser: empty user id")
	return a.Store.Team().GetTeamsForUser(ctx, userID)
}

// AddTeamMember adds userID to a team on actorID's behalf.
//
// Anyone may join an OPEN team themselves; that is what open means. Adding
// somebody else, or joining an invite-only team, requires the actor to be a
// member already, mirroring AddChannelMember. Until that check existed the
// handler took no actor at all, so ANY authenticated caller could add ANYONE
// to ANY team, and team membership is what AddChannelMember consults to
// authorize joining an open channel.
func (a *App) AddTeamMember(ctx context.Context, teamID, userID, actorID string) (*model.TeamMember, error) {
	assert.That(actorID != "", "App.AddTeamMember: empty actor id")
	assert.That(a.Store != nil && a.Hub != nil, "App.AddTeamMember: nil store or hub")
	team, err := a.Store.Team().Get(ctx, teamID)
	if err != nil {
		return nil, err
	}
	assert.That(team != nil, "App.AddTeamMember: nil team without an error")
	if userID != actorID || team.Type != model.TeamOpen {
		if authErr := a.requireTeamMember(ctx, teamID, actorID); authErr != nil {
			return nil, authErr
		}
	}
	if userErr := a.requireUsersExist(ctx, "App.AddTeamMember", []string{userID}); userErr != nil {
		return nil, userErr
	}

	member := &model.TeamMember{
		TeamID: teamID,
		UserID: userID,
	}
	saved, err := a.Store.Team().SaveMember(ctx, member)
	if err != nil {
		return nil, err
	}
	assert.That(saved != nil, "App.AddTeamMember: nil member without an error")
	a.Hub.NotifyTeamMembershipChanged(userID, teamID, true)
	return saved, nil
}

// RemoveTeamMember removes a user from a team on actorID's behalf.
//
// Leaving is always allowed; removing SOMEBODY ELSE requires system_admin. The
// same split RemoveChannelMember makes, and for the same reason: a member who
// can evict their peers is an admin in everything but name.
func (a *App) RemoveTeamMember(ctx context.Context, teamID, userID, actorID string) error {
	assert.That(actorID != "", "App.RemoveTeamMember: empty actor id")
	assert.That(a.Store != nil && a.Hub != nil, "App.RemoveTeamMember: nil store or hub")
	assert.That(a.keto != nil, "App.RemoveTeamMember: nil keto client")
	if err := a.requireTeamMember(ctx, teamID, actorID); err != nil {
		return err
	}
	if userID != actorID {
		if err := a.requireSystemAdmin(ctx, actorID, "App.RemoveTeamMember"); err != nil {
			return err
		}
	}

	if err := a.Store.Team().RemoveMember(ctx, teamID, userID); err != nil {
		return err
	}
	a.Hub.NotifyTeamMembershipChanged(userID, teamID, false)

	// Channel membership is the access check, so leaving the team must
	// leave its channels too. It did not: a user removed from a team kept
	// reading and posting in every channel they had been in.
	channelIDs, err := a.Store.Channel().RemoveMemberFromTeam(ctx, teamID, userID)
	if err != nil {
		return err
	}
	for _, channelID := range channelIDs {
		assert.That(channelID != "", "App.RemoveTeamMember: store returned an empty channel id")
		a.Hub.NotifyMembershipChanged(userID, channelID, false)
		a.logKeto(a.keto.DeleteRelation(ctx, model.KetoNamespaceChannel, channelID, model.KetoRelationMember, userID), channelID, userID)
		a.publishUserRemoved(ctx, channelID, userID)
	}
	return nil
}

// GetTeamMembers lists a team's members on actor's behalf. Only members and
// system admins may read the roster.
func (a *App) GetTeamMembers(ctx context.Context, teamID string, actor *model.User, page, perPage int) ([]*model.TeamMember, error) {
	assert.That(actor != nil, "App.GetTeamMembers: nil actor")
	assert.That(page >= 0 && perPage >= 1, "App.GetTeamMembers: pagination not clamped")
	if !actor.IsSystemAdmin() {
		if err := a.requireTeamMember(ctx, teamID, actor.ID); err != nil {
			return nil, err
		}
	}
	return a.Store.Team().GetMembers(ctx, teamID, page, perPage)
}
