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
		Roles:  "team_admin team_user",
	}
	if _, err := a.Store.Team().SaveMember(ctx, member); err != nil {
		return nil, err
	}
	a.Hub.NotifyTeamMembershipChanged(saved.CreatorID, saved.ID, true)

	return saved, nil
}

// GetTeam retrieves a team by ID.
func (a *App) GetTeam(ctx context.Context, id string) (*model.Team, error) {
	return a.Store.Team().Get(ctx, id)
}

// UpdateTeam updates a team.
func (a *App) UpdateTeam(ctx context.Context, team *model.Team) (*model.Team, error) {
	return a.Store.Team().Update(ctx, team)
}

// DeleteTeam soft-deletes a team.
func (a *App) DeleteTeam(ctx context.Context, id string) error {
	return a.Store.Team().Delete(ctx, id, model.GetMillis())
}

// GetAllTeams retrieves all teams with pagination.
func (a *App) GetAllTeams(ctx context.Context, page, perPage int) ([]*model.Team, error) {
	return a.Store.Team().GetAll(ctx, page, perPage)
}

// GetTeamsForUser retrieves teams that a user is a member of.
func (a *App) GetTeamsForUser(ctx context.Context, userID string) ([]*model.Team, error) {
	return a.Store.Team().GetTeamsForUser(ctx, userID)
}

// AddTeamMember adds userID to a team on actorID's behalf.
//
// The actor must already be a member, mirroring AddChannelMember. Until this
// check existed the handler took no actor at all, so ANY authenticated caller
// could add ANYONE to ANY team — and team membership is what
// AddChannelMember consults to authorize joining an open channel, so this was
// also the way into every open channel on that team.
func (a *App) AddTeamMember(ctx context.Context, teamID, userID, actorID string) (*model.TeamMember, error) {
	if err := a.requireTeamMember(ctx, teamID, actorID); err != nil {
		return nil, err
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
	return nil
}

// GetTeamMembers retrieves members of a team.
func (a *App) GetTeamMembers(ctx context.Context, teamID string, page, perPage int) ([]*model.TeamMember, error) {
	return a.Store.Team().GetMembers(ctx, teamID, page, perPage)
}
