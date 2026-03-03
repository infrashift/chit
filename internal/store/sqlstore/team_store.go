package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/model"
)

type SqlTeamStore struct {
	sqlStore *SqlStore
}

func (s *SqlTeamStore) Save(team *model.Team) (*model.Team, error) {
	team.PreSave()
	if err := team.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO teams (id, name, display_name, description, type, creator_id, create_at, update_at, delete_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := s.sqlStore.pool.Exec(context.Background(), query,
		team.ID, team.Name, team.DisplayName, team.Description, team.Type,
		team.CreatorID, team.CreateAt, team.UpdateAt, team.DeleteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save team: %w", err)
	}

	return team, nil
}

func (s *SqlTeamStore) Get(id string) (*model.Team, error) {
	query := `SELECT id, name, display_name, description, type, creator_id, create_at, update_at, delete_at
		FROM teams WHERE id = $1 AND delete_at = 0`

	team := &model.Team{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, id).Scan(
		&team.ID, &team.Name, &team.DisplayName, &team.Description, &team.Type,
		&team.CreatorID, &team.CreateAt, &team.UpdateAt, &team.DeleteAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlTeamStore.Get", id)
		}
		return nil, fmt.Errorf("get team: %w", err)
	}

	return team, nil
}

func (s *SqlTeamStore) GetByName(name string) (*model.Team, error) {
	query := `SELECT id, name, display_name, description, type, creator_id, create_at, update_at, delete_at
		FROM teams WHERE name = $1 AND delete_at = 0`

	team := &model.Team{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, name).Scan(
		&team.ID, &team.Name, &team.DisplayName, &team.Description, &team.Type,
		&team.CreatorID, &team.CreateAt, &team.UpdateAt, &team.DeleteAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlTeamStore.GetByName", name)
		}
		return nil, fmt.Errorf("get team by name: %w", err)
	}

	return team, nil
}

func (s *SqlTeamStore) Update(team *model.Team) (*model.Team, error) {
	team.PreUpdate()

	query := `UPDATE teams SET name = $1, display_name = $2, description = $3, type = $4, update_at = $5
		WHERE id = $6 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(context.Background(), query,
		team.Name, team.DisplayName, team.Description, team.Type, team.UpdateAt, team.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, model.NewNotFoundError("SqlTeamStore.Update", team.ID)
	}

	return team, nil
}

func (s *SqlTeamStore) Delete(id string, deleteAt int64) error {
	query := `UPDATE teams SET delete_at = $1, update_at = $1 WHERE id = $2 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(context.Background(), query, deleteAt, id)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.NewNotFoundError("SqlTeamStore.Delete", id)
	}
	return nil
}

func (s *SqlTeamStore) GetAll(page, perPage int) ([]*model.Team, error) {
	query := `SELECT id, name, display_name, description, type, creator_id, create_at, update_at, delete_at
		FROM teams WHERE delete_at = 0 ORDER BY display_name LIMIT $1 OFFSET $2`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("get all teams: %w", err)
	}
	defer rows.Close()

	return scanTeams(rows)
}

func (s *SqlTeamStore) GetTeamsForUser(userID string) ([]*model.Team, error) {
	query := `SELECT t.id, t.name, t.display_name, t.description, t.type, t.creator_id, t.create_at, t.update_at, t.delete_at
		FROM teams t
		INNER JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1 AND t.delete_at = 0 AND tm.delete_at = 0
		ORDER BY t.display_name`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, userID)
	if err != nil {
		return nil, fmt.Errorf("get teams for user: %w", err)
	}
	defer rows.Close()

	return scanTeams(rows)
}

func (s *SqlTeamStore) SaveMember(member *model.TeamMember) (*model.TeamMember, error) {
	member.PreSave()
	if err := member.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO team_members (team_id, user_id, roles, create_at, delete_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (team_id, user_id) DO UPDATE SET roles = EXCLUDED.roles
		RETURNING team_id, user_id, roles, create_at, delete_at`
	err := s.sqlStore.pool.QueryRow(context.Background(), query,
		member.TeamID, member.UserID, member.Roles, member.CreateAt, member.DeleteAt,
	).Scan(&member.TeamID, &member.UserID, &member.Roles, &member.CreateAt, &member.DeleteAt)
	if err != nil {
		return nil, fmt.Errorf("save team member: %w", err)
	}

	return member, nil
}

func (s *SqlTeamStore) RemoveMember(teamID, userID string) error {
	query := `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`
	_, err := s.sqlStore.pool.Exec(context.Background(), query, teamID, userID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	return nil
}

func (s *SqlTeamStore) GetMembers(teamID string, page, perPage int) ([]*model.TeamMember, error) {
	query := `SELECT team_id, user_id, roles, create_at, delete_at
		FROM team_members WHERE team_id = $1 AND delete_at = 0
		ORDER BY create_at LIMIT $2 OFFSET $3`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, teamID, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("get team members: %w", err)
	}
	defer rows.Close()

	var members []*model.TeamMember
	for rows.Next() {
		m := &model.TeamMember{}
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Roles, &m.CreateAt, &m.DeleteAt); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *SqlTeamStore) GetMember(teamID, userID string) (*model.TeamMember, error) {
	query := `SELECT team_id, user_id, roles, create_at, delete_at
		FROM team_members WHERE team_id = $1 AND user_id = $2 AND delete_at = 0`

	m := &model.TeamMember{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, teamID, userID).Scan(
		&m.TeamID, &m.UserID, &m.Roles, &m.CreateAt, &m.DeleteAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlTeamStore.GetMember", teamID+"/"+userID)
		}
		return nil, fmt.Errorf("get team member: %w", err)
	}

	return m, nil
}

func scanTeams(rows pgx.Rows) ([]*model.Team, error) {
	var teams []*model.Team
	for rows.Next() {
		t := &model.Team{}
		if err := rows.Scan(
			&t.ID, &t.Name, &t.DisplayName, &t.Description, &t.Type,
			&t.CreatorID, &t.CreateAt, &t.UpdateAt, &t.DeleteAt,
		); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}
