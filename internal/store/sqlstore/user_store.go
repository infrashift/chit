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

func (s *SqlUserStore) Save(ctx context.Context, user *model.User) (*model.User, error) {
	user.PreSave()
	if err := user.IsValid(); err != nil {
		return nil, err
	}

	// NULLIF on all three of the OPTIONAL identity columns: each is UNIQUE, and
	// Postgres allows many NULLs but only one ''. oauth_client_id is NULL for a
	// person; kratos_id and email are NULL for a machine. Writing '' instead
	// would let exactly one machine exist and refuse the second with a unique
	// violation naming a column nobody set.
	query := `INSERT INTO users (id, kratos_id, username, display_name, email, roles, actor_type, oauth_client_id, create_at, update_at, delete_at)
		VALUES ($1, NULLIF($2, ''), $3, $4, NULLIF($5, ''), $6, $7, NULLIF($8, ''), $9, $10, $11)`
	_, err := s.sqlStore.pool.Exec(ctx, query,
		user.ID, user.KratosID, user.Username, user.DisplayName, user.Email,
		user.Roles, user.ActorType, user.OAuthClientID, user.CreateAt, user.UpdateAt, user.DeleteAt,
	)
	if err != nil {
		return nil, fmt.Errorf("save user: %w", err)
	}

	return user, nil
}

func (s *SqlUserStore) Get(ctx context.Context, id string) (*model.User, error) {
	return s.getBy(ctx, "id", id)
}

func (s *SqlUserStore) GetByKratosID(ctx context.Context, kratosID string) (*model.User, error) {
	return s.getBy(ctx, "kratos_id", kratosID)
}

func (s *SqlUserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.getBy(ctx, "username", username)
}

func (s *SqlUserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.getBy(ctx, "email", email)
}

// GetByOAuthClientID resolves the machine actor bound to an OAuth2 client.
// Unlike GetByKratosID there is no just-in-time provisioning fallback: an
// unrecognised client must fail authentication rather than mint a user.
func (s *SqlUserStore) GetByOAuthClientID(ctx context.Context, clientID string) (*model.User, error) {
	if clientID == "" {
		return nil, model.NewNotFoundError("SqlUserStore.GetByOAuthClientID", clientID)
	}
	return s.getBy(ctx, "oauth_client_id", clientID)
}

func (s *SqlUserStore) getBy(ctx context.Context, column, value string) (*model.User, error) {
	query := fmt.Sprintf(
		`SELECT id, COALESCE(kratos_id, ''), username, display_name, COALESCE(email, ''), roles, actor_type,
			COALESCE(oauth_client_id, ''), create_at, update_at, delete_at
		FROM users WHERE %s = $1 AND delete_at = 0`, column,
	)
	user := &model.User{}
	err := s.sqlStore.pool.QueryRow(ctx, query, value).Scan(
		&user.ID, &user.KratosID, &user.Username, &user.DisplayName, &user.Email,
		&user.Roles, &user.ActorType, &user.OAuthClientID,
		&user.CreateAt, &user.UpdateAt, &user.DeleteAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, model.NewNotFoundError("SqlUserStore.getBy", value)
		}
		return nil, fmt.Errorf("get user by %s: %w", column, err)
	}

	return user, nil
}

func (s *SqlUserStore) Update(ctx context.Context, user *model.User) (*model.User, error) {
	user.PreUpdate()

	query := `UPDATE users SET username = $1, display_name = $2, email = NULLIF($3, ''), roles = $4, actor_type = $5, update_at = $6
		WHERE id = $7 AND delete_at = 0`
	tag, err := s.sqlStore.pool.Exec(ctx, query,
		user.Username, user.DisplayName, user.Email, user.Roles, user.ActorType, user.UpdateAt, user.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, model.NewNotFoundError("SqlUserStore.Update", user.ID)
	}

	return user, nil
}

func (s *SqlUserStore) Search(ctx context.Context, term string, page, perPage int) ([]*model.User, error) {
	query := `SELECT id, COALESCE(kratos_id, ''), username, display_name, COALESCE(email, ''), roles, actor_type, create_at, update_at, delete_at
		FROM users
		WHERE delete_at = 0 AND (username ILIKE $1 OR display_name ILIKE $1 OR email ILIKE $1)
		ORDER BY username
		LIMIT $2 OFFSET $3`

	like := "%" + term + "%"
	rows, err := s.sqlStore.pool.Query(ctx, query, like, perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	return scanUsers(rows)
}

func (s *SqlUserStore) GetByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	query := `SELECT id, COALESCE(kratos_id, ''), username, display_name, COALESCE(email, ''), roles, actor_type, create_at, update_at, delete_at
		FROM users WHERE id = ANY($1) AND delete_at = 0`

	rows, err := s.sqlStore.pool.Query(ctx, query, ids)
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
			&u.Roles, &u.ActorType, &u.CreateAt, &u.UpdateAt, &u.DeleteAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
