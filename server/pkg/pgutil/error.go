package pgutil

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsAliasConflict checks if the error is a PostgreSQL unique constraint violation for an alias.
func IsAliasConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "alias")
}
