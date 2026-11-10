// Package postgrestest gives integration tests a migrated test database.
package postgrestest

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/umartahir93/olivaw-assignment/migrations"
)

// defaultURL is the test database from docker-compose.yml.
const defaultURL = "postgres://fleet:fleet@localhost:5432/fleet_test?sslmode=disable"

// Open connects to the test database (TEST_DATABASE_URL, or the one from
// docker-compose.yml), applies the migrations and closes the connection when
// the test ends.
func Open(t *testing.T) *sqlx.DB {
	t.Helper()
	url := defaultURL
	if v, ok := os.LookupEnv("TEST_DATABASE_URL"); ok {
		url = v
	}
	db, err := migrations.Open(context.Background(), url)
	if err != nil {
		t.Fatalf("open test database (start it with `make db-up`): %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	if err := migrations.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return db
}

// Reset empties every table, so each test case starts from nothing.
func Reset(t *testing.T, db *sqlx.DB) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `TRUNCATE robots, gateways`); err != nil {
		t.Fatalf("reset test database: %v", err)
	}
}
