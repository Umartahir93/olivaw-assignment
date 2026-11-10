// Package domain holds robots and their rules. It does not know about gRPC,
// HTTP or storage.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"

	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

var (
	// ErrNotFound means the robot does not exist.
	ErrNotFound = errors.New("robot not found")
	// ErrOtherGateway means a robot was reported by a gateway it does not
	// belong to. Robots never move between gateways.
	ErrOtherGateway = errors.New("robot belongs to another gateway")
)

// Robot is a robot connected to a gateway, known from the gateway's scans.
type Robot struct {
	ID           uuid.UUID
	SerialNumber string // business key
	GatewayID    uuid.UUID
	FirstSeenAt  time.Time  // when a scan first reported it
	LastSeenAt   *time.Time // nil until its first measurement
}

// Connectivity derives the robot's connectivity at now, by the same rules
// as a gateway's.
func (r Robot) Connectivity(now time.Time) gatewaydomain.Connectivity {
	return gatewaydomain.ConnectivityAt(r.LastSeenAt, now)
}
