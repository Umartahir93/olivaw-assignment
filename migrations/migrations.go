// Package migrations holds the SQL migrations of the database, embedded in
// the binary so they run wherever the binary runs.
package migrations

import "embed"

// FS contains the migration files, applied in the order of their names.
//
//go:embed *.sql
var FS embed.FS
