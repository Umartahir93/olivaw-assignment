// Package domain holds gateways and their rules. It does not know about
// gRPC, HTTP or storage.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound means the gateway does not exist.
	ErrNotFound = errors.New("gateway not found")
	// ErrAlreadyExists means a gateway with this serial number is already
	// registered.
	ErrAlreadyExists = errors.New("gateway already exists")
)

// OfflineAfter is how long the system waits for data before it considers a
// gateway or robot offline.
const OfflineAfter = 5 * time.Minute

// Connectivity is whether the system hears from a gateway or robot.
type Connectivity string

// Connectivity values.
const (
	ConnectivityUnknown Connectivity = "unknown" // no contact yet
	ConnectivityOnline  Connectivity = "online"  // contact within OfflineAfter
	ConnectivityOffline Connectivity = "offline" // no contact for longer
)

// ConnectivityAt derives connectivity at now from the last contact; a nil
// lastSeen means no contact yet.
func ConnectivityAt(lastSeen *time.Time, now time.Time) Connectivity {
	switch {
	case lastSeen == nil:
		return ConnectivityUnknown
	case now.Sub(*lastSeen) <= OfflineAfter:
		return ConnectivityOnline
	default:
		return ConnectivityOffline
	}
}

// Gateway is the edge device at a site that robots are connected to.
type Gateway struct {
	ID           uuid.UUID
	SerialNumber string
	Location     string
	Name         *string
	RegisteredAt time.Time
	// nil until the gateway's first call
	LastSeenAt *time.Time
}

// Connectivity derives the gateway's connectivity at now.
func (g Gateway) Connectivity(now time.Time) Connectivity {
	return ConnectivityAt(g.LastSeenAt, now)
}
