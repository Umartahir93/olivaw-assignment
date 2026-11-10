package domain

import (
	"time"

	"github.com/google/uuid"
)

// MaxTimeRange is the longest time range one read of measurements or faults
// may cover.
const MaxTimeRange = 24 * time.Hour

// Activity is what a robot is doing.
type Activity string

// Activity values.
const (
	ActivityUnknown  Activity = "unknown" // not reported
	ActivityIdle     Activity = "idle"
	ActivityMoving   Activity = "moving"
	ActivityCharging Activity = "charging"
	ActivityError    Activity = "error" // the robot cannot work
)

// Position is where a robot is on a site's map.
type Position struct {
	MapID      string
	X, Y       float64 // metres from the map's origin
	HeadingRad float64 // counter-clockwise from the map's x axis
}

// Measurement is one robot's state at one moment. It is never changed once
// stored; a robot's measurements are identified by MeasuredAt.
type Measurement struct {
	RobotID          uuid.UUID
	BatteryPct       *float64  // nil if not reported
	Position         *Position // nil if not reported
	Activity         Activity
	ActiveFaultCodes []string
	MeasuredAt       time.Time // the robot's clock
	ReceivedAt       time.Time // the system's clock
}

// Severity is how badly a fault affects a robot.
type Severity string

// Severity values.
const (
	SeverityUnknown  Severity = "unknown"  // not reported
	SeverityWarning  Severity = "warning"  // the robot keeps working
	SeverityCritical Severity = "critical" // the robot cannot work until someone helps
)

// Fault is one occurrence of a problem a robot detected. A robot's faults
// are identified by Code and OccurredAt.
type Fault struct {
	RobotID    uuid.UUID
	Code       string
	Severity   Severity
	Message    string
	OccurredAt time.Time // the robot's clock
	ReceivedAt time.Time // the system's clock
}
