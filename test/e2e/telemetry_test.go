//go:build e2e

package e2e

import (
	"math"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
)

// TestTelemetry: a gateway sends one measurement per robot per call, and
// faults through a separate call. Every accepted report marks the robot as
// seen.
func TestTelemetry(t *testing.T) {
	// givenScannedRobot registers GW-1 with robot SN-1 and returns the
	// gateway and SN-1's robotId.
	givenScannedRobot := func(t *testing.T, sc scenario) (gateway, string) {
		t.Helper()
		var (
			gw gateway
			id string
		)
		step(t, "Given gateway GW-1 is registered and scanned robot SN-1", func(t *testing.T) {
			gw = registerGateway(t, sc.serial("GW-1"))
			id = gw.mustScan(t, sc.serial("SN-1"))[sc.serial("SN-1")]
		})
		return gw, id
	}

	t.Run("A measurement marks the robot online", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())

		step(t, "And SN-1 has not been heard from", func(t *testing.T) {
			if c := getRobot(t, robotID).Connectivity; c != fleetv1.ConnectivityUnknown {
				t.Fatalf("connectivity = %s, want unknown", c)
			}
		})
		step(t, "When GW-1 reports a measurement for SN-1", func(t *testing.T) {
			if err := gw.reportMeasurement(t, measurement(robotID, 80)); err != nil {
				t.Fatalf("report measurement: %v", err)
			}
		})
		step(t, "Then SN-1 is online", func(t *testing.T) {
			r := getRobot(t, robotID)
			if r.Connectivity != fleetv1.ConnectivityOnline || r.LastSeenAt.Null {
				t.Errorf("connectivity = %s, lastSeenAt null = %t; want online, seen", r.Connectivity, r.LastSeenAt.Null)
			}
		})
	})

	t.Run("An invalid measurement is rejected", func(t *testing.T) {
		tests := []struct {
			name   string
			breaks func(m *gatewayv1.Measurement)
		}{
			{
				name:   "battery above 100",
				breaks: func(m *gatewayv1.Measurement) { m.BatteryPct = proto.Float64(101) },
			},
			{
				name:   "position not finite",
				breaks: func(m *gatewayv1.Measurement) { m.Position.X = math.Inf(1) },
			},
			{
				name:   "no timestamp",
				breaks: func(m *gatewayv1.Measurement) { m.MeasuredAt = nil },
			},
			{
				name:   "robot_id not a UUID",
				breaks: func(m *gatewayv1.Measurement) { m.RobotId = "robot-1" },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				gw, robotID := givenScannedRobot(t, newScenario())
				var err error

				step(t, "When GW-1 reports a measurement for SN-1 with "+tt.name, func(t *testing.T) {
					m := measurement(robotID, 80)
					tt.breaks(m)
					err = gw.reportMeasurement(t, m)
				})
				step(t, "Then it is rejected with InvalidArgument", func(t *testing.T) {
					if code := status.Code(err); code != codes.InvalidArgument {
						t.Errorf("code = %s, want %s (error: %v)", code, codes.InvalidArgument, err)
					}
				})
				step(t, "And SN-1 is still not heard from", func(t *testing.T) {
					if c := getRobot(t, robotID).Connectivity; c != fleetv1.ConnectivityUnknown {
						t.Errorf("connectivity = %s, want unknown", c)
					}
				})
			})
		}
	})

	t.Run("A gateway cannot report for another gateway's robot", func(t *testing.T) {
		sc := newScenario()
		_, robotID := givenScannedRobot(t, sc)
		var (
			other gateway
			err   error
		)

		step(t, "And gateway GW-2 is registered", func(t *testing.T) {
			other = registerGateway(t, sc.serial("GW-2"))
		})
		step(t, "When GW-2 reports a measurement for SN-1", func(t *testing.T) {
			err = other.reportMeasurement(t, measurement(robotID, 80))
		})
		step(t, "Then it is rejected with PermissionDenied", func(t *testing.T) {
			if code := status.Code(err); code != codes.PermissionDenied {
				t.Errorf("code = %s, want %s (error: %v)", code, codes.PermissionDenied, err)
			}
		})
	})

	t.Run("A fault marks the robot online", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())

		step(t, "When GW-1 reports fault E_MOTOR for SN-1", func(t *testing.T) {
			if err := gw.reportFault(t, robotID, "E_MOTOR"); err != nil {
				t.Fatalf("report fault: %v", err)
			}
		})
		step(t, "Then SN-1 is online", func(t *testing.T) {
			if c := getRobot(t, robotID).Connectivity; c != fleetv1.ConnectivityOnline {
				t.Errorf("connectivity = %s, want online", c)
			}
		})
	})

	t.Run("A fault needs a code", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())
		var err error

		step(t, "When GW-1 reports a fault without a code for SN-1", func(t *testing.T) {
			err = gw.reportFault(t, robotID, "")
		})
		step(t, "Then it is rejected with InvalidArgument", func(t *testing.T) {
			if code := status.Code(err); code != codes.InvalidArgument {
				t.Errorf("code = %s, want %s (error: %v)", code, codes.InvalidArgument, err)
			}
		})
	})
}
