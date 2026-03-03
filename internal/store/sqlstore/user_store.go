package sqlstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/infrashift/chit/internal/model"
)

type SqlUserStore struct {
	sqlStore *SqlStore
}

func (s *SqlUserStore) Save(user *model.User) (*model.User, error) {
	user.PreSave()
	if err := user.IsValid(); err != nil {
		return nil, err
	}

	query := `INSERT INTO users (id, kratos_id, username, display_name, email, roles, create_at, update_at, delete_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := s.sqlStore.pool.Exec(context.Background(), query,
		user.ID, user.KratosID, user.Username, user.DisplayName, user.Email,
		user.Roles, user.CreateAt, user.UpdateAt, user.DeleteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save user: %w", err)
	}

	return user, nil
}

func (s *SqlUserStore) Get(id string) (*model.User, error) {
	return s.getBy("id", id)
}

func (s *SqlUserStore) GetByKratosID(kratosID string) (*model.User, error) {
	return s.getBy("kratos_id", kratosID)
}

func (s *SqlUserStore) GetByUsername(username string) (*model.User, error) {
	return s.getBy("username", username)
}

func (s *SqlUserStore) GetByEmail(email string) (*model.User, error) {
	return s.getBy("email", email)
}

func (s *SqlUserStore) getBy(column, value string) (*model.User, error) {
	query := fmt.Sprintf(
		`SELECT id, kratos_id, username, display_name, email, roles, create_at, update_at, delete_at
		FROM users WHERE %s = $1 AND delete_at = 0`, column,
	)
	user := &model.User{}
	err := s.sqlStore.pool.QueryRow(context.Background(), query, value).Scan(
		&user.ID, &user.KratosID, &user.Username, &user.DisplayName, &user.Email,
		&user.Roles, &user.CreateAt, &user.UpdateAt, &user.DeleteAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlUserStore.getBy", value)
		}
		return nil, fmt.Errorf("get user by %s: %w", column, err)
	}

	return user, nil
}

func (s *SqlUserStore) Update(user *model.User) (*model.User, error) {
	user.PreUpdate()

	query := `UPDATE users SET username = $1, display_name = $2, email = $3, roles = $4, update_at = $5
		WHERE id = $6 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(context.Background(), query,
		user.Username, user.DisplayName, user.Email, user.Roles, user.UpdateAt, user.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, model.NewNotFoundError("SqlUserStore.Update", user.ID)
	}

	return user, nil
}

func (s *SqlUserStore) Search(term string, page, perPage int) ([]*model.User, error) {
	query := `SELECT id, kratos_id, username, display_name, email, roles, create_at, update_at, delete_at
		FROM users
		WHERE delete_at = 0 AND (username ILIKE $1 OR display_name ILIKE $1 OR email ILIKE $1)
		ORDER BY username
		LIMIT $2 OFFSET $3`

	like := "%" + term + "%"
	rows, err := s.sqlStore.pool.Query(context.Background(), query, like, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	return scanUsers(rows)
}

func (s *SqlUserStore) GetByIDs(ids []string) ([]*model.User, error) {
	query := `SELECT id, kratos_id, username, display_name, email, roles, create_at, update_at, delete_at
		FROM users WHERE id = ANY($1) AND delete_at = 0`

	rows, err := s.sqlStore.pool.Query(context.Background(), query, ids)
	if err != nil {
		return nil, fmt.Errorf("get users by ids: %w", err)
	}
	defer rows.Close()

	return scanUsers(rows)
}

func scanUsers(rows pgx.Rows) ([]*model.User, error) {
	var users []*model.User
	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(
			&u.ID, &u.KratosID, &u.Username, &u.DisplayName, &u.Email,
			&u.Roles, &u.CreateAt, &u.UpdateAt, &u.DeleteAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
