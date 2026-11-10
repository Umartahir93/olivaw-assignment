package repository

import (
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// robot is a row of the robots table. The db tags name its columns.
type robot struct {
	ID           uuid.UUID  `db:"id"`
	SerialNumber string     `db:"serial_number"`
	GatewayID    uuid.UUID  `db:"gateway_id"`
	FirstSeenAt  time.Time  `db:"first_seen_at"`
	LastSeenAt   *time.Time `db:"last_seen_at"`
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
	return rb
}
