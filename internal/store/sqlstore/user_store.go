package sqlstore

import (
	"context"
	"errors"
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
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, ''), $6, $7, NULLIF($8, ''), $9, $10, $11)`
	_, err := s.sqlStore.pool.Exec(ctx, query,
		user.ID, user.KratosID, user.Username, user.DisplayName, user.Email,
		user.Roles, user.ActorType, user.OAuthClientID, user.CreateAt, user.UpdateAt, user.DeleteAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.NewConflictError("SqlUserStore.Save", "username, email or identity is already taken")
		}
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

// GetByOAuthClientID resolves the machine actor bound to an OAuth2 client.
// Unlike GetByKratosID there is no just-in-time provisioning fallback: an
// unrecognised client must fail authentication rather than mint a user.
func (s *SqlUserStore) GetByOAuthClientID(ctx context.Context, clientID string) (*model.User, error) {
	if clientID == "" {
		return nil, model.NewNotFoundError("SqlUserStore.GetByOAuthClientID", clientID)
	}
	return s.getBy(ctx, "oauth_client_id", clientID)
}

// userColumns is the one SELECT list for users, and scanUser its one reader.
// The nullable identity columns come back as ” rather than NULL, with the
// uuid cast to text first: COALESCE(kratos_id, ”) resolves to uuid and fails
// on ” (see 75af3a0). Every reader shares this so a column added here reaches
// all of them, rather than some readers silently dropping it.
const userColumns = `id, COALESCE(kratos_id::text, ''), username, display_name, COALESCE(email, ''),
	roles, actor_type, COALESCE(oauth_client_id, ''), create_at, update_at, delete_at`

func scanUser(row pgx.Row) (*model.User, error) {
	u := &model.User{}
	err := row.Scan(
		&u.ID, &u.KratosID, &u.Username, &u.DisplayName, &u.Email,
		&u.Roles, &u.ActorType, &u.OAuthClientID,
		&u.CreateAt, &u.UpdateAt, &u.DeleteAt,
	)
	return u, err
}

func (s *SqlUserStore) getBy(ctx context.Context, column, value string) (*model.User, error) {
	// id and kratos_id are uuid columns. A value that is not a UUID can match
	// nothing, and sent as-is Postgres rejects the cast (SQLSTATE 22P02),
	// which surfaced as a 500 for GET /users/not-a-uuid.
	if (column == "id" || column == "kratos_id") && !model.IsValidID(value) {
		return nil, model.NewNotFoundError("SqlUserStore.getBy", value)
	}
	query := fmt.Sprintf(`SELECT `+userColumns+` FROM users WHERE %s = $1 AND delete_at = 0`, column)
	user, err := scanUser(s.sqlStore.pool.QueryRow(ctx, query, value))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
		if isUniqueViolation(err) {
			return nil, model.NewConflictError("SqlUserStore.Update", "username or email is already taken")
		}
		return nil, fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, model.NewNotFoundError("SqlUserStore.Update", user.ID)
	}

	return user, nil
}

// Search matches username and display name. Not email: results are sanitized,
// but which users match still answered "does anyone's address contain X",
// one guess at a time, which is enough to recover an address.
func (s *SqlUserStore) Search(ctx context.Context, term string, page, perPage int) ([]*model.User, error) {
	query := `SELECT ` + userColumns + `
		FROM users
		WHERE delete_at = 0 AND (username ILIKE $1 ESCAPE '\' OR display_name ILIKE $1 ESCAPE '\')
		ORDER BY username
		LIMIT $2 OFFSET $3`

	rows, err := s.sqlStore.pool.Query(ctx, query, likePattern(term), perPage, page*perPage)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	return scanUsers(rows)
}

func (s *SqlUserStore) GetByIDs(ctx context.Context, ids []string) ([]*model.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = ANY($1) AND delete_at = 0`

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
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
