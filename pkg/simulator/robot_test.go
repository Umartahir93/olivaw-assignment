package simulator

import (
	"math/rand/v2"
	"testing"
	"time"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
)

func newTestRobot(seed uint64) *Robot {
	return NewRobot("SN-TEST", rand.New(rand.NewPCG(seed, seed)))
}

func TestReadingsStayPlausible(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		r := newTestRobot(seed)
		now := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)

		// Ten simulated hours, long enough for several charging cycles.
		for i := 0; i < 36_000; i++ {
			now = now.Add(time.Second)
			reading := r.Step(time.Second, now)

			if b := reading.GetBatteryPct(); b < 0 || b > 100 {
				t.Fatalf("seed %d, step %d: battery %v outside 0..100", seed, i, b)
			}
			if p := reading.GetPosition(); p.GetX() < 0 || p.GetX() > mapSize || p.GetY() < 0 || p.GetY() > mapSize {
				t.Fatalf("seed %d, step %d: position (%v, %v) outside the map", seed, i, p.GetX(), p.GetY())
			}
			if !reading.GetMeasuredAt().AsTime().Equal(now) {
				t.Fatalf("seed %d, step %d: measured_at %v, want %v", seed, i, reading.GetMeasuredAt().AsTime(), now)
			}
		}
	}
}

func TestLowBatteryDrivesToChargerAndCharges(t *testing.T) {
	r := newTestRobot(1)
	r.battery = lowBattery - 1

	reading := r.Step(time.Second, time.Now())
	if reading.GetActivity() != gatewayv1.Activity_ACTIVITY_MOVING || !r.toCharger {
		t.Fatalf("activity = %v, toCharger = %v; want moving to the charger", reading.GetActivity(), r.toCharger)
	}

	charged := false
	for i := 0; i < 3600 && !charged; i++ {
		reading = r.Step(time.Second, time.Now())
		charged = reading.GetActivity() == gatewayv1.Activity_ACTIVITY_CHARGING
	}
	if !charged {
		t.Fatal("robot never started charging")
	}
	if r.x != chargerX || r.y != chargerY {
		t.Errorf("charging at (%v, %v), want the charger at (%v, %v)", r.x, r.y, chargerX, chargerY)
	}
}

func TestChargingStopsAtFull(t *testing.T) {
	r := newTestRobot(1)
	r.activity = gatewayv1.Activity_ACTIVITY_CHARGING
	r.battery = 99.5

	reading := r.Step(time.Second, time.Now())

	if reading.GetBatteryPct() != 100 || reading.GetActivity() != gatewayv1.Activity_ACTIVITY_IDLE {
		t.Fatalf("battery %v, activity %v; want 100 and idle", reading.GetBatteryPct(), reading.GetActivity())
	}
}
