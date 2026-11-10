// Package simulator models robots that produce plausible telemetry: they
// drive between random points on a map, use battery while doing so, and
// return to a charger when the battery runs low.
package simulator

import (
	"math"
	"math/rand/v2"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	telemetryv1 "github.com/umartahir93/olivaw-assignment/gen/telemetry/v1"
)

const (
	mapID   = "sim-floor-1"
	mapSize = 50.0 // metres; the map spans 0..mapSize on both axes

	speed       = 1.0  // metres per second
	drainMoving = 0.2  // battery percent per second while moving
	drainIdle   = 0.02 // battery percent per second while idle
	chargeRate  = 1.0  // battery percent per second while charging
	lowBattery  = 20.0 // below this, the robot drives to the charger

	// Chance per second that an idle robot starts driving somewhere.
	startRate = 0.3
)

// The charger is in the map's origin.
const chargerX, chargerY = 0.0, 0.0

// Robot is one simulated robot. It is not safe for concurrent use.
type Robot struct {
	ID string

	x, y, heading float64
	battery       float64
	activity      telemetryv1.Activity

	targetX, targetY float64
	toCharger        bool

	rng *rand.Rand
}

// NewRobot returns an idle robot at a random position with 50 to 100 %
// battery. rng drives all its random choices.
func NewRobot(id string, rng *rand.Rand) *Robot {
	return &Robot{
		ID:       id,
		x:        rng.Float64() * mapSize,
		y:        rng.Float64() * mapSize,
		battery:  50 + rng.Float64()*50,
		activity: telemetryv1.Activity_ACTIVITY_IDLE,
		rng:      rng,
	}
}

// Step advances the robot by dt and returns its state as a reading taken at
// now.
func (r *Robot) Step(dt time.Duration, now time.Time) *telemetryv1.TelemetryReading {
	secs := dt.Seconds()

	switch r.activity {
	case telemetryv1.Activity_ACTIVITY_CHARGING:
		r.battery += chargeRate * secs
		if r.battery >= 100 {
			r.battery = 100
			r.activity = telemetryv1.Activity_ACTIVITY_IDLE
		}
	case telemetryv1.Activity_ACTIVITY_MOVING:
		r.battery -= drainMoving * secs
		if r.moveTowardTarget(speed * secs) {
			r.arrive()
		}
	default:
		r.battery -= drainIdle * secs
		if r.rng.Float64() < startRate*secs {
			r.driveTo(r.rng.Float64()*mapSize, r.rng.Float64()*mapSize, false)
		}
	}

	if r.battery < lowBattery && !r.toCharger && r.activity != telemetryv1.Activity_ACTIVITY_CHARGING {
		r.driveTo(chargerX, chargerY, true)
	}
	r.battery = max(r.battery, 0)

	return &telemetryv1.TelemetryReading{
		BatteryPct: proto.Float64(r.battery),
		Position: &telemetryv1.Position{
			MapId:      mapID,
			X:          r.x,
			Y:          r.y,
			HeadingRad: r.heading,
		},
		Activity:   r.activity,
		RecordedAt: timestamppb.New(now),
	}
}

func (r *Robot) driveTo(x, y float64, toCharger bool) {
	r.targetX, r.targetY, r.toCharger = x, y, toCharger
	r.activity = telemetryv1.Activity_ACTIVITY_MOVING
}

// moveTowardTarget moves up to distance metres and reports whether the
// robot reached its target.
func (r *Robot) moveTowardTarget(distance float64) bool {
	dx, dy := r.targetX-r.x, r.targetY-r.y
	remaining := math.Hypot(dx, dy)
	if remaining <= distance {
		r.x, r.y = r.targetX, r.targetY
		return true
	}
	r.heading = math.Atan2(dy, dx)
	r.x += dx / remaining * distance
	r.y += dy / remaining * distance
	return false
}

func (r *Robot) arrive() {
	if r.toCharger {
		r.toCharger = false
		r.activity = telemetryv1.Activity_ACTIVITY_CHARGING
		return
	}
	r.activity = telemetryv1.Activity_ACTIVITY_IDLE
}
