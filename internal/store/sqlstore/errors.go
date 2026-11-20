package sqlstore

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/errlog"
	"github.com/infrashift/chit/internal/model"
)

// rollback rolls tx back. Defer it right after Begin: after a Commit the
// transaction is closed and pgx.ErrTxClosed is expected, so only another
// error is logged.
func rollback(ctx context.Context, tx pgx.Tx) {
	assert.That(tx != nil, "rollback: nil transaction")
	if err := tx.Rollback(ctx); !errors.Is(err, pgx.ErrTxClosed) {
		errlog.Warn(err, "rollback transaction")
	}
}

// isUniqueViolation reports whether err is a Postgres unique_violation
// (SQLSTATE 23505), which callers map to a 409 instead of a 500.
func isUniqueViolation(err error) bool {
	assert.That(err != nil, "isUniqueViolation: nil error")
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// validIDs reports whether every id is a UUID. A lookup by one that is not
// can match nothing; sent to Postgres it fails the uuid cast (SQLSTATE
// 22P02), which surfaced as a 500 rather than a 404 for a malformed id from
// a path or a request body.
func validIDs(ids ...string) bool {
	assert.That(len(ids) > 0, "validIDs: no ids")
	for _, id := range ids {
		if !model.IsValidID(id) {
			return false
		}
	}
	return true
}

// likePattern turns a user's search term into an ILIKE pattern matching it
// anywhere, with the term's own % and _ matched literally rather than as
// wildcards. Use it with ESCAPE '\'.
func likePattern(term string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	pattern := "%" + r.Replace(term) + "%"
	assert.That(len(pattern) >= len(term)+2, "likePattern: pattern lost characters of the term")
	assert.That(strings.HasPrefix(pattern, "%") && strings.HasSuffix(pattern, "%"), "likePattern: pattern must match anywhere")
	return pattern
}
