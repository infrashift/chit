package app

import (
	"context"

	"github.com/infrashift/chit/internal/model"
)

// CreateTeam creates a new team and adds the creator as a member.
func (a *App) CreateTeam(ctx context.Context, team *model.Team) (*model.Team, error) {
	saved, err := a.Store.Team().Save(ctx, team)
	if err != nil {
		return nil, err
	}

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
	team, err := a.Store.Team().Get(ctx, id)
	if err != nil {
		return nil, err
	}
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
	team, err := a.Store.Team().Get(ctx, id)
	if err != nil {
		return nil, err
	}
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
	return a.Store.Team().Update(ctx, team)
}

// DeleteTeam soft-deletes a team on actor's behalf. Requires team_admin or
// system_admin.
func (a *App) DeleteTeam(ctx context.Context, id string, actor *model.User) error {
	if _, err := a.Store.Team().Get(ctx, id); err != nil {
		return err
	}
	if err := a.requireTeamAdmin(ctx, id, actor, "App.DeleteTeam"); err != nil {
		return err
	}
	now := model.GetMillis()
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
		a.publishChannelDeleted(ctx, channelID)
	}
	return nil
}

// GetAllTeams lists the teams actor can see: every team for a system admin,
// otherwise open teams plus the invite-only teams actor belongs to.
func (a *App) GetAllTeams(ctx context.Context, actor *model.User, page, perPage int) ([]*model.Team, error) {
	visibleTo := actor.ID
	if actor.IsSystemAdmin() {
		visibleTo = ""
	}
	return a.Store.Team().GetAll(ctx, visibleTo, page, perPage)
}

// GetTeamsForUser retrieves teams that a user is a member of.
func (a *App) GetTeamsForUser(ctx context.Context, userID string) ([]*model.Team, error) {
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
	team, err := a.Store.Team().Get(ctx, teamID)
	if err != nil {
		return nil, err
	}
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
	a.Hub.NotifyTeamMembershipChanged(userID, teamID, true)
	return saved, nil
}

// RemoveTeamMember removes a user from a team on actorID's behalf.
//
// Leaving is always allowed; removing SOMEBODY ELSE requires system_admin. The
// same split RemoveChannelMember makes, and for the same reason: a member who
// can evict their peers is an admin in everything but name.
func (a *App) RemoveTeamMember(ctx context.Context, teamID, userID, actorID string) error {
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
		a.Hub.NotifyMembershipChanged(userID, channelID, false)
		a.logKeto(a.keto.DeleteRelation(ctx, model.KetoNamespaceChannel, channelID, model.KetoRelationMember, userID), channelID, userID)
		a.publishUserRemoved(ctx, channelID, userID)
	}
	return nil
}

// GetTeamMembers lists a team's members on actor's behalf. Only members and
// system admins may read the roster.
func (a *App) GetTeamMembers(ctx context.Context, teamID string, actor *model.User, page, perPage int) ([]*model.TeamMember, error) {
	if !actor.IsSystemAdmin() {
		if err := a.requireTeamMember(ctx, teamID, actor.ID); err != nil {
			return nil, err
		}
	}
	return a.Store.Team().GetMembers(ctx, teamID, page, perPage)
}
