package repository

import (
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

// gateway is a row of the gateways table. The db tags name its columns.
type gateway struct {
	ID           uuid.UUID  `db:"id"`
	SerialNumber string     `db:"serial_number"`
	Location     string     `db:"location"`
	Name         *string    `db:"name"`
	RegisteredAt time.Time  `db:"registered_at"`
	LastSeenAt   *time.Time `db:"last_seen_at"`
}

// gatewayFromDomain turns a domain gateway into a row.
func gatewayFromDomain(g domain.Gateway) gateway {
	return gateway{
		ID:           g.ID,
		SerialNumber: g.SerialNumber,
		Location:     g.Location,
		Name:         g.Name,
		RegisteredAt: g.RegisteredAt,
		LastSeenAt:   g.LastSeenAt,
	}
}

// toDomain turns the row into a domain gateway, with times in UTC.
func (r gateway) toDomain() domain.Gateway {
	g := domain.Gateway{
		ID:           r.ID,
		SerialNumber: r.SerialNumber,
		Location:     r.Location,
		Name:         r.Name,
		RegisteredAt: r.RegisteredAt.UTC(),
	}
	if r.LastSeenAt != nil {
		lastSeen := r.LastSeenAt.UTC()
		g.LastSeenAt = &lastSeen
	}
	return g
}
