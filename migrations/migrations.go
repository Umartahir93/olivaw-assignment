// Package migrations connects to the database and brings its schema up to
// date. The SQL migrations are embedded in the binary, so they run wherever
// the binary runs.
package migrations

import "embed"

// FS contains the migration files, applied in the order of their names.
//
//go:embed *.sql
var FS embed.FS
