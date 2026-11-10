// Package repository stores robots, their measurements and their faults in
// PostgreSQL.
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

// Queries. Every robot select returns robotColumns from robotsFrom.
const (
	// A robot comes with its newest measurement, if any (the latest_
	// columns). It is last seen when that measurement arrived, or when a scan
	// last reported it, whichever is later.
	robotColumns = `r.id, r.serial_number, r.gateway_id, r.first_seen_at,
		GREATEST(r.last_seen_at, m.received_at) AS last_seen_at,
		m.measured_at        AS latest_measured_at,
		m.received_at        AS latest_received_at,
		m.battery_pct        AS latest_battery_pct,
		m.map_id             AS latest_map_id,
		m.x                  AS latest_x,
		m.y                  AS latest_y,
		m.heading_rad        AS latest_heading_rad,
		m.activity           AS latest_activity,
		m.active_fault_codes AS latest_active_fault_codes`

	// robotsFrom joins every robot to its newest measurement.
	robotsFrom = `robots r
		LEFT JOIN LATERAL (
			SELECT *
			FROM measurements
			WHERE robot_id = r.id
			ORDER BY measured_at DESC
			LIMIT 1
		) m ON true`

	insertRobotSQL = `
		INSERT INTO robots (id, serial_number, gateway_id, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5)`

	selectRobotByIDSQL = `
		SELECT ` + robotColumns + `
		FROM ` + robotsFrom + `
		WHERE r.id = $1`

	selectRobotsBySerialSQL = `
		SELECT ` + robotColumns + `
		FROM ` + robotsFrom + `
		WHERE r.serial_number = ANY($1)
		ORDER BY r.first_seen_at, r.id`

	selectRobotsByGatewaySQL = `
		SELECT ` + robotColumns + `
		FROM ` + robotsFrom + `
		WHERE r.gateway_id = $1
		ORDER BY r.first_seen_at, r.id`

	updateRobotLastSeenSQL = `
		UPDATE robots
		SET last_seen_at = $2
		WHERE id = $1`

	measurementColumns = `robot_id, measured_at, received_at, battery_pct,
		map_id, x, y, heading_rad, activity, active_fault_codes`

	// A measurement that is already stored is not stored again.
	insertMeasurementSQL = `
		INSERT INTO measurements (` + measurementColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (robot_id, measured_at) DO NOTHING`

	selectMeasurementsSQL = `
		SELECT ` + measurementColumns + `
		FROM measurements
		WHERE robot_id = $1 AND measured_at >= $2 AND measured_at < $3
		ORDER BY measured_at`

	faultColumns = `robot_id, code, occurred_at, received_at, severity, message`

	// A fault that is already stored is not stored again.
	insertFaultSQL = `
		INSERT INTO faults (` + faultColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (robot_id, code, occurred_at) DO NOTHING`

	selectFaultsSQL = `
		SELECT ` + faultColumns + `
		FROM faults
		WHERE robot_id = $1 AND occurred_at >= $2 AND occurred_at < $3
		ORDER BY occurred_at, code`
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
	insertMeasurement     *sqlx.Stmt
	selectMeasurements    *sqlx.Stmt
	insertFault           *sqlx.Stmt
	selectFaults          *sqlx.Stmt
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
		{&r.insertMeasurement, insertMeasurementSQL},
		{&r.selectMeasurements, selectMeasurementsSQL},
		{&r.insertFault, insertFaultSQL},
		{&r.selectFaults, selectFaultsSQL},
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
		r.insertMeasurement,
		r.selectMeasurements,
		r.insertFault,
		r.selectFaults,
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

// CreateMeasurement stores m and returns it. A measurement of the same
// robot with the same MeasuredAt is already stored: then nothing changes.
func (r *Repository) CreateMeasurement(ctx context.Context, m domain.Measurement) (domain.Measurement, error) {
	row := measurementFromDomain(m)
	_, err := r.insertMeasurement.ExecContext(
		ctx,
		row.RobotID,
		row.MeasuredAt,
		row.ReceivedAt,
		row.BatteryPct,
		row.MapID,
		row.X,
		row.Y,
		row.HeadingRad,
		row.Activity,
		[]string(row.ActiveFaultCodes),
	)
	if err != nil {
		return domain.Measurement{}, err
	}
	return row.toDomain(), nil
}

// ListMeasurements returns the robot's measurements taken from from
// (inclusive) to to (exclusive), oldest first.
func (r *Repository) ListMeasurements(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Measurement, error) {
	var rows []measurement
	if err := r.selectMeasurements.SelectContext(ctx, &rows, robotID, from, to); err != nil {
		return nil, err
	}

	measurements := make([]domain.Measurement, 0, len(rows))
	for _, row := range rows {
		measurements = append(measurements, row.toDomain())
	}
	return measurements, nil
}

// CreateFault stores f and returns it. A fault of the same robot with the
// same Code and OccurredAt is already stored: then nothing changes.
func (r *Repository) CreateFault(ctx context.Context, f domain.Fault) (domain.Fault, error) {
	row := faultFromDomain(f)
	_, err := r.insertFault.ExecContext(
		ctx,
		row.RobotID,
		row.Code,
		row.OccurredAt,
		row.ReceivedAt,
		row.Severity,
		row.Message,
	)
	if err != nil {
		return domain.Fault{}, err
	}
	return row.toDomain(), nil
}

// ListFaults returns the robot's faults that occurred from from (inclusive)
// to to (exclusive), oldest first.
func (r *Repository) ListFaults(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Fault, error) {
	var rows []fault
	if err := r.selectFaults.SelectContext(ctx, &rows, robotID, from, to); err != nil {
		return nil, err
	}

	faults := make([]domain.Fault, 0, len(rows))
	for _, row := range rows {
		faults = append(faults, row.toDomain())
	}
	return faults, nil
}

// robotsToDomain turns rows into domain robots.
func robotsToDomain(rows []robot) []domain.Robot {
	robots := make([]domain.Robot, 0, len(rows))
	for _, row := range rows {
		robots = append(robots, row.toDomain())
	}
	return robots
}
