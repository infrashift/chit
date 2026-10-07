package sqlstore

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// isUniqueViolation reports whether err is a Postgres unique_violation
// (SQLSTATE 23505), which callers map to a 409 instead of a 500.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// likePattern turns a user's search term into an ILIKE pattern matching it
// anywhere, with the term's own % and _ matched literally rather than as
// wildcards. Use it with ESCAPE '\'.
func likePattern(term string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(term) + "%"
}
