package app

import (
	"github.com/infrashift/chit/internal/model"
)

// CreateTeam creates a new team and adds the creator as a member.
func (a *App) CreateTeam(team *model.Team) (*model.Team, error) {
	saved, err := a.Store.Team().Save(team)
	if err != nil {
		return nil, err
	}

	member := &model.TeamMember{
		TeamID: saved.ID,
		UserID: saved.CreatorID,
		Roles:  "team_admin team_user",
	}
	if _, err := a.Store.Team().SaveMember(member); err != nil {
		return nil, err
	}

	return saved, nil
}

// GetTeam retrieves a team by ID.
func (a *App) GetTeam(id string) (*model.Team, error) {
	return a.Store.Team().Get(id)
}

// UpdateTeam updates a team.
func (a *App) UpdateTeam(team *model.Team) (*model.Team, error) {
	return a.Store.Team().Update(team)
}

// DeleteTeam soft-deletes a team.
func (a *App) DeleteTeam(id string) error {
	return a.Store.Team().Delete(id, model.GetMillis())
}

// GetAllTeams retrieves all teams with pagination.
func (a *App) GetAllTeams(page, perPage int) ([]*model.Team, error) {
	return a.Store.Team().GetAll(page, perPage)
}

// GetTeamsForUser retrieves teams that a user is a member of.
func (a *App) GetTeamsForUser(userID string) ([]*model.Team, error) {
	return a.Store.Team().GetTeamsForUser(userID)
}

// AddTeamMember adds a user to a team.
func (a *App) AddTeamMember(teamID, userID string) (*model.TeamMember, error) {
	member := &model.TeamMember{
		TeamID: teamID,
		UserID: userID,
	}
	return a.Store.Team().SaveMember(member)
}

// RemoveTeamMember removes a user from a team.
func (a *App) RemoveTeamMember(teamID, userID string) error {
	return a.Store.Team().RemoveMember(teamID, userID)
}

// GetTeamMembers retrieves members of a team.
func (a *App) GetTeamMembers(teamID string, page, perPage int) ([]*model.TeamMember, error) {
	return a.Store.Team().GetMembers(teamID, page, perPage)
}
