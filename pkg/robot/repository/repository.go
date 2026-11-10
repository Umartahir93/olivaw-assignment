// Package repository stores robots in PostgreSQL.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/umartahir93/olivaw-assignment/utils"

	robotservice "github.com/umartahir93/olivaw-assignment/pkg/robot"
	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// Queries. Every select returns robotColumns, in that order.
const (
	robotColumns = `id, serial_number, gateway_id, first_seen_at, last_seen_at`

	insertRobotSQL = `
		INSERT INTO robots (id, serial_number, gateway_id, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5)`

	selectRobotByIDSQL = `
		SELECT ` + robotColumns + `
		FROM robots
		WHERE id = $1`

	selectRobotsBySerialSQL = `
		SELECT ` + robotColumns + `
		FROM robots
		WHERE serial_number = ANY($1)
		ORDER BY first_seen_at, id`

	selectRobotsByGatewaySQL = `
		SELECT ` + robotColumns + `
		FROM robots
		WHERE gateway_id = $1
		ORDER BY first_seen_at, id`

	updateRobotLastSeenSQL = `
		UPDATE robots
		SET last_seen_at = $2
		WHERE id = $1`
)

// Repository implements robot.Repository on PostgreSQL with statements
// prepared on the database server.
type Repository struct {
	db                    *sqlx.DB // for transactions
	insertRobot           *sqlx.Stmt
	selectRobotByID       *sqlx.Stmt
	selectRobotsBySerial  *sqlx.Stmt
	selectRobotsByGateway *sqlx.Stmt
	updateRobotLastSeen   *sqlx.Stmt
}

var _ robotservice.Repository = (*Repository)(nil)

// New prepares the repository's statements on db. Close releases them.
func New(ctx context.Context, db *sqlx.DB) (*Repository, error) {
	r := &Repository{db: db}

	statements := []struct {
		stmt  **sqlx.Stmt
		query string
	}{
		{&r.insertRobot, insertRobotSQL},
		{&r.selectRobotByID, selectRobotByIDSQL},
		{&r.selectRobotsBySerial, selectRobotsBySerialSQL},
		{&r.selectRobotsByGateway, selectRobotsByGatewaySQL},
		{&r.updateRobotLastSeen, updateRobotLastSeenSQL},
	}

	for _, s := range statements {
		stmt, err := db.PreparexContext(ctx, s.query)
		if err != nil {
			_ = r.Close()
			return nil, err
		}
		*s.stmt = stmt
	}

	return r, nil
}

// Close releases the prepared statements.
func (r *Repository) Close() error {
	var errs []error

	for _, stmt := range []*sqlx.Stmt{
		r.insertRobot,
		r.selectRobotByID,
		r.selectRobotsBySerial,
		r.selectRobotsByGateway,
		r.updateRobotLastSeen,
	} {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// CreateRobots stores robots in one transaction, so either all of them are
// stored or none, and returns them as stored.
func (r *Repository) CreateRobots(ctx context.Context, robots []domain.Robot) ([]domain.Robot, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op once the transaction is committed

	insert := tx.StmtxContext(ctx, r.insertRobot)
	created := make([]domain.Robot, 0, len(robots))
	for _, rb := range robots {
		robotDB := robotFromDomain(rb)
		_, err := insert.ExecContext(
			ctx,
			robotDB.ID,
			robotDB.SerialNumber,
			robotDB.GatewayID,
			robotDB.FirstSeenAt,
			robotDB.LastSeenAt,
		)
		if err != nil {
			return nil, err
		}
		created = append(created, robotDB.toDomain())
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return created, nil
}

// GetRobot returns the robot with this ID, or domain.ErrNotFound.
func (r *Repository) GetRobot(ctx context.Context, id uuid.UUID) (domain.Robot, error) {
	var robotDB robot
	err := r.selectRobotByID.GetContext(ctx, &robotDB, id)
	switch {
	case utils.IsNoRows(err):
		return domain.Robot{}, domain.ErrNotFound
	case err != nil:
		return domain.Robot{}, err
	}

	return robotDB.toDomain(), nil
}

// GetRobotsBySerial returns the stored robots with these serial numbers,
// from any gateway, oldest first. Unknown serial numbers are left out.
func (r *Repository) GetRobotsBySerial(ctx context.Context, serialNumbers []string) ([]domain.Robot, error) {
	var rows []robot

	if err := r.selectRobotsBySerial.SelectContext(ctx, &rows, serialNumbers); err != nil {
		return nil, err
	}

	return robotsToDomain(rows), nil
}

// ListRobots returns a gateway's robots, oldest first.
func (r *Repository) ListRobots(ctx context.Context, gatewayID uuid.UUID) ([]domain.Robot, error) {
	var rows []robot

	if err := r.selectRobotsByGateway.SelectContext(ctx, &rows, gatewayID); err != nil {
		return nil, err
	}

	return robotsToDomain(rows), nil
}

// UpdateRobotLastSeen sets the robot's LastSeenAt to at and returns the
// robot, or domain.ErrNotFound.
func (r *Repository) UpdateRobotLastSeen(ctx context.Context, id uuid.UUID, at time.Time) (domain.Robot, error) {
	result, err := r.updateRobotLastSeen.ExecContext(ctx, id, at)
	if err != nil {
		return domain.Robot{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Robot{}, err
	}
	if affected == 0 {
		return domain.Robot{}, domain.ErrNotFound
	}

	return r.GetRobot(ctx, id)
}

// robotsToDomain turns rows into domain robots.
func robotsToDomain(rows []robot) []domain.Robot {
	robots := make([]domain.Robot, 0, len(rows))
	for _, row := range rows {
		robots = append(robots, row.toDomain())
	}
	return robots
}
