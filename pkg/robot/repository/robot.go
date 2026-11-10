package repository

import (
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// robot is a row of the robots table, joined to the robot's newest
// measurement (all null if there is none). The db tags name its columns.
type robot struct {
	ID           uuid.UUID  `db:"id"`
	SerialNumber string     `db:"serial_number"`
	GatewayID    uuid.UUID  `db:"gateway_id"`
	FirstSeenAt  time.Time  `db:"first_seen_at"`
	LastSeenAt   *time.Time `db:"last_seen_at"`

	LatestMeasuredAt       *time.Time `db:"latest_measured_at"`
	LatestReceivedAt       *time.Time `db:"latest_received_at"`
	LatestBatteryPct       *float64   `db:"latest_battery_pct"`
	LatestMapID            *string    `db:"latest_map_id"`
	LatestX                *float64   `db:"latest_x"`
	LatestY                *float64   `db:"latest_y"`
	LatestHeadingRad       *float64   `db:"latest_heading_rad"`
	LatestActivity         *string    `db:"latest_activity"`
	LatestActiveFaultCodes textArray  `db:"latest_active_fault_codes"`
}

// robotFromDomain turns a domain robot into a row.
func robotFromDomain(r domain.Robot) robot {
	return robot{
		ID:           r.ID,
		SerialNumber: r.SerialNumber,
		GatewayID:    r.GatewayID,
		FirstSeenAt:  r.FirstSeenAt,
		LastSeenAt:   r.LastSeenAt,
	}
}

// toDomain turns the row into a domain robot, with times in UTC.
func (r robot) toDomain() domain.Robot {
	rb := domain.Robot{
		ID:           r.ID,
		SerialNumber: r.SerialNumber,
		GatewayID:    r.GatewayID,
		FirstSeenAt:  r.FirstSeenAt.UTC(),
	}
	if r.LastSeenAt != nil {
		lastSeen := r.LastSeenAt.UTC()
		rb.LastSeenAt = &lastSeen
	}
	if r.LatestMeasuredAt != nil && r.LatestReceivedAt != nil && r.LatestActivity != nil {
		latest := measurement{
			RobotID:          r.ID,
			MeasuredAt:       *r.LatestMeasuredAt,
			ReceivedAt:       *r.LatestReceivedAt,
			BatteryPct:       r.LatestBatteryPct,
			MapID:            r.LatestMapID,
			X:                r.LatestX,
			Y:                r.LatestY,
			HeadingRad:       r.LatestHeadingRad,
			Activity:         *r.LatestActivity,
			ActiveFaultCodes: r.LatestActiveFaultCodes,
		}.toDomain()
		rb.Latest = &latest
	}
	return rb
}
