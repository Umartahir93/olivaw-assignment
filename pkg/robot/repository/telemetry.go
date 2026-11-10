package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// measurement is a row of the measurements table. The db tags name its
// columns. A position that was not reported is all null.
type measurement struct {
	RobotID          uuid.UUID `db:"robot_id"`
	MeasuredAt       time.Time `db:"measured_at"`
	ReceivedAt       time.Time `db:"received_at"`
	BatteryPct       *float64  `db:"battery_pct"`
	MapID            *string   `db:"map_id"`
	X                *float64  `db:"x"`
	Y                *float64  `db:"y"`
	HeadingRad       *float64  `db:"heading_rad"`
	Activity         string    `db:"activity"`
	ActiveFaultCodes textArray `db:"active_fault_codes"`
}

// measurementFromDomain turns a domain measurement into a row.
func measurementFromDomain(m domain.Measurement) measurement {
	row := measurement{
		RobotID:          m.RobotID,
		MeasuredAt:       m.MeasuredAt,
		ReceivedAt:       m.ReceivedAt,
		BatteryPct:       m.BatteryPct,
		Activity:         string(m.Activity),
		ActiveFaultCodes: textArray(m.ActiveFaultCodes),
	}
	if row.ActiveFaultCodes == nil {
		row.ActiveFaultCodes = textArray{}
	}
	if p := m.Position; p != nil {
		row.MapID, row.X, row.Y, row.HeadingRad = &p.MapID, &p.X, &p.Y, &p.HeadingRad
	}
	return row
}

// toDomain turns the row into a domain measurement, with times in UTC.
func (m measurement) toDomain() domain.Measurement {
	out := domain.Measurement{
		RobotID:          m.RobotID,
		BatteryPct:       m.BatteryPct,
		Activity:         domain.Activity(m.Activity),
		ActiveFaultCodes: []string(m.ActiveFaultCodes),
		MeasuredAt:       m.MeasuredAt.UTC(),
		ReceivedAt:       m.ReceivedAt.UTC(),
	}
	if m.MapID != nil && m.X != nil && m.Y != nil && m.HeadingRad != nil {
		out.Position = &domain.Position{MapID: *m.MapID, X: *m.X, Y: *m.Y, HeadingRad: *m.HeadingRad}
	}
	return out
}

// fault is a row of the faults table. The db tags name its columns.
type fault struct {
	RobotID    uuid.UUID `db:"robot_id"`
	Code       string    `db:"code"`
	OccurredAt time.Time `db:"occurred_at"`
	ReceivedAt time.Time `db:"received_at"`
	Severity   string    `db:"severity"`
	Message    string    `db:"message"`
}

// faultFromDomain turns a domain fault into a row.
func faultFromDomain(f domain.Fault) fault {
	return fault{
		RobotID:    f.RobotID,
		Code:       f.Code,
		OccurredAt: f.OccurredAt,
		ReceivedAt: f.ReceivedAt,
		Severity:   string(f.Severity),
		Message:    f.Message,
	}
}

// toDomain turns the row into a domain fault, with times in UTC.
func (f fault) toDomain() domain.Fault {
	return domain.Fault{
		RobotID:    f.RobotID,
		Code:       f.Code,
		Severity:   domain.Severity(f.Severity),
		Message:    f.Message,
		OccurredAt: f.OccurredAt.UTC(),
		ReceivedAt: f.ReceivedAt.UTC(),
	}
}

// pgTypes decodes PostgreSQL values database/sql cannot, such as arrays.
var pgTypes = pgtype.NewMap()

// textArray is a text[] column. database/sql reads arrays as text, so Scan
// decodes them with pgx; writes pass the []string to pgx, which encodes it.
type textArray []string

// Scan implements sql.Scanner.
func (a *textArray) Scan(src any) error {
	return pgTypes.SQLScanner((*[]string)(a)).Scan(src)
}
