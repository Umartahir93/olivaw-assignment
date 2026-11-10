//go:build e2e

package e2e

import (
	"testing"

	"github.com/google/uuid"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
)

// TestFleetLifecycle follows one gateway from registration to reporting
// telemetry, checking every step the way a fleet manager sees it.
func TestFleetLifecycle(t *testing.T) {
	sc := newScenario()
	var (
		gw  gateway
		ids map[string]string
	)

	step(t, "When I register gateway GW-1 at Munich HQ named Main hall", func(t *testing.T) {
		name := "Main hall"
		created, ok := register(t, sc.serial("GW-1"), "Munich HQ", &name).(*fleetv1.GatewayHeaders)
		if !ok {
			t.Fatal("gateway not created")
		}
		gw = gateway{id: created.Response.GatewayId}
	})
	step(t, "Then GW-1 is registered and has never called", func(t *testing.T) {
		g := getGateway(t, gw.id)
		if g.SerialNumber != sc.serial("GW-1") || g.Location != "Munich HQ" || g.Name.Value != "Main hall" {
			t.Errorf("gateway = %s at %s named %q", g.SerialNumber, g.Location, g.Name.Value)
		}
		if g.Connectivity != fleetv1.ConnectivityUnknown || !g.LastSeenAt.Null {
			t.Errorf("connectivity = %s, lastSeenAt null = %t; want unknown, never seen", g.Connectivity, g.LastSeenAt.Null)
		}
	})

	step(t, "When GW-1 scans robots SN-1 and SN-2", func(t *testing.T) {
		ids = gw.mustScan(t, sc.serials("SN-1", "SN-2")...)
	})
	step(t, "Then every scanned robot has a robotId", func(t *testing.T) {
		for _, serial := range sc.serials("SN-1", "SN-2") {
			if _, err := uuid.Parse(ids[serial]); err != nil {
				t.Errorf("robot %s: robotId %q is not a UUID", serial, ids[serial])
			}
		}
	})
	step(t, "And GW-1 is online", func(t *testing.T) {
		if c := getGateway(t, gw.id).Connectivity; c != fleetv1.ConnectivityOnline {
			t.Errorf("connectivity = %s, want online", c)
		}
	})
	step(t, "And the fleet manager sees SN-1 and SN-2 under GW-1, online, without measurements yet", func(t *testing.T) {
		robots := gatewayRobots(t, gw.id)
		if len(robots) != 2 {
			t.Fatalf("gateway has %d robots, want 2", len(robots))
		}
		for _, serial := range sc.serials("SN-1", "SN-2") {
			r := robots[serial]
			if r.RobotId.String() != ids[serial] || r.GatewayId != gw.id || r.Connectivity != fleetv1.ConnectivityOnline {
				t.Errorf("robot %s = %s of %s, %s; want %s of %s, online", serial, r.RobotId, r.GatewayId, r.Connectivity, ids[serial], gw.id)
			}
			if !r.Latest.Null {
				t.Errorf("robot %s: latest measurement set, want none yet", serial)
			}
		}
	})

	sent := measurement(ids[sc.serial("SN-1")], 80)
	step(t, "When GW-1 reports a measurement for SN-1", func(t *testing.T) {
		if err := gw.reportMeasurement(t, sent); err != nil {
			t.Fatalf("report measurement: %v", err)
		}
	})
	step(t, "Then the fleet manager sees it in SN-1's measurements", func(t *testing.T) {
		got := robotMeasurements(t, ids[sc.serial("SN-1")])
		if len(got) != 1 {
			t.Fatalf("SN-1 has %d measurements, want 1", len(got))
		}
		if !got[0].MeasuredAt.Equal(sent.GetMeasuredAt().AsTime()) || got[0].BatteryPct.Value != 80 {
			t.Errorf("measurement = %v at %s, want battery 80 at %s", got[0].BatteryPct.Value, got[0].MeasuredAt, sent.GetMeasuredAt().AsTime())
		}
	})
	step(t, "And as SN-1's latest state, while SN-2 has none", func(t *testing.T) {
		if latest := getRobot(t, ids[sc.serial("SN-1")]).Latest; latest.Null || latest.Value.Activity != fleetv1.ActivityMoving {
			t.Errorf("SN-1 latest = %+v, want the measurement, moving", latest)
		}
		if latest := getRobot(t, ids[sc.serial("SN-2")]).Latest; !latest.Null {
			t.Errorf("SN-2 latest = %+v, want none", latest)
		}
	})

	step(t, "When GW-1 reports fault E_MOTOR for SN-2", func(t *testing.T) {
		if err := gw.reportFault(t, fault(ids[sc.serial("SN-2")], "E_MOTOR")); err != nil {
			t.Fatalf("report fault: %v", err)
		}
	})
	step(t, "Then the fleet manager sees it in SN-2's faults", func(t *testing.T) {
		got := robotFaults(t, ids[sc.serial("SN-2")])
		if len(got) != 1 || got[0].Code != "E_MOTOR" || got[0].Severity != fleetv1.FaultSeverityCritical {
			t.Errorf("SN-2 faults = %+v, want one critical E_MOTOR", got)
		}
	})
}
