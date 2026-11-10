// Package utils holds small helpers shared across packages.
package utils

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsNoRows reports whether err means a query found no row.
func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// IsUniqueViolation reports whether err means an insert or update broke a
// unique constraint.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
