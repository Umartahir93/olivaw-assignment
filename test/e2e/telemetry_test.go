//go:build e2e

package e2e

import (
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
)

// TestTelemetry: a gateway sends one measurement per robot per call, and
// faults through a separate call. The system stores both, once each, and a
// fleet manager reads them over REST.
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

	t.Run("A measurement is stored with every value", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())
		sent := measurement(robotID, 80)
		sent.ActiveFaultCodes = []string{"E_LIDAR"}

		step(t, "When GW-1 reports a measurement for SN-1", func(t *testing.T) {
			if err := gw.reportMeasurement(t, sent); err != nil {
				t.Fatalf("report measurement: %v", err)
			}
		})
		step(t, "Then it is in SN-1's measurements as sent", func(t *testing.T) {
			want := []fleetv1.Measurement{
				{
					BatteryPct:       fleetv1.NewOptNilFloat64(80),
					Position:         fleetv1.NewOptNilPosition(fleetv1.Position{MapId: "floor-1", X: 12.5, Y: 3, HeadingRad: 1.57}),
					Activity:         fleetv1.ActivityMoving,
					ActiveFaultCodes: []string{"E_LIDAR"},
					MeasuredAt:       sent.GetMeasuredAt().AsTime(),
				},
			}
			if diff := cmp.Diff(want, robotMeasurements(t, robotID), equateTime); diff != "" {
				t.Errorf("measurements mismatch (-want +got):\n%s", diff)
			}
		})
		step(t, "And it is SN-1's latest state", func(t *testing.T) {
			latest := getRobot(t, robotID).Latest
			if latest.Null || !latest.Value.MeasuredAt.Equal(sent.GetMeasuredAt().AsTime()) {
				t.Errorf("latest = %+v, want the measurement", latest)
			}
		})
	})

	t.Run("A measurement sent twice is stored once", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())
		sent := measurement(robotID, 80)

		step(t, "When GW-1 reports the same measurement twice", func(t *testing.T) {
			for range 2 {
				if err := gw.reportMeasurement(t, sent); err != nil {
					t.Fatalf("report measurement: %v", err)
				}
			}
		})
		step(t, "Then SN-1 has it once", func(t *testing.T) {
			if got := robotMeasurements(t, robotID); len(got) != 1 {
				t.Errorf("SN-1 has %d measurements, want 1", len(got))
			}
		})
	})

	t.Run("Measurements are read by time range", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())
		end := now()
		older := measurement(robotID, 90)
		older.MeasuredAt = timestamppb.New(end.Add(-2 * time.Hour))
		newer := measurement(robotID, 70)
		newer.MeasuredAt = timestamppb.New(end.Add(-10 * time.Minute))

		step(t, "Given GW-1 reported a measurement 2 hours ago and one 10 minutes ago", func(t *testing.T) {
			for _, m := range []*gatewayv1.Measurement{older, newer} {
				if err := gw.reportMeasurement(t, m); err != nil {
					t.Fatalf("report measurement: %v", err)
				}
			}
		})
		step(t, "When the fleet manager asks for the last hour, only the newer one is listed", func(t *testing.T) {
			res, err := rest.ListRobotMeasurements(t.Context(), fleetv1.ListRobotMeasurementsParams{
				RobotId: uuid.MustParse(robotID),
				From:    fleetv1.NewOptDateTime(end.Add(-time.Hour)),
				To:      fleetv1.NewOptDateTime(end),
			})
			if err != nil {
				t.Fatalf("list measurements: %v", err)
			}
			list, ok := res.(*fleetv1.MeasurementList)
			if !ok || len(list.Items) != 1 || list.Items[0].BatteryPct.Value != 70 {
				t.Errorf("got %+v, want only the measurement with battery 70", res)
			}
		})
		step(t, "And without a range, both are listed, oldest first", func(t *testing.T) {
			got := robotMeasurements(t, robotID)
			if len(got) != 2 || got[0].BatteryPct.Value != 90 || got[1].BatteryPct.Value != 70 {
				t.Errorf("got %+v, want battery 90 then 70", got)
			}
		})
		step(t, "And a range longer than 24 hours is rejected with 400", func(t *testing.T) {
			res, err := rest.ListRobotMeasurements(t.Context(), fleetv1.ListRobotMeasurementsParams{
				RobotId: uuid.MustParse(robotID),
				From:    fleetv1.NewOptDateTime(end.Add(-25 * time.Hour)),
				To:      fleetv1.NewOptDateTime(end),
			})
			if err != nil {
				t.Fatalf("list measurements: %v", err)
			}
			if _, ok := res.(*fleetv1.ListRobotMeasurementsBadRequest); !ok {
				t.Errorf("got %T, want a bad request", res)
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
				step(t, "And nothing is stored", func(t *testing.T) {
					if got := robotMeasurements(t, robotID); len(got) != 0 {
						t.Errorf("SN-1 has %d measurements, want none", len(got))
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
		step(t, "And nothing is stored", func(t *testing.T) {
			if got := robotMeasurements(t, robotID); len(got) != 0 {
				t.Errorf("SN-1 has %d measurements, want none", len(got))
			}
		})
	})

	t.Run("A fault is stored", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())
		sent := fault(robotID, "E_MOTOR")

		step(t, "When GW-1 reports fault E_MOTOR for SN-1", func(t *testing.T) {
			if err := gw.reportFault(t, sent); err != nil {
				t.Fatalf("report fault: %v", err)
			}
		})
		step(t, "Then it is in SN-1's faults as sent", func(t *testing.T) {
			want := []fleetv1.Fault{
				{
					Code:       "E_MOTOR",
					Severity:   fleetv1.FaultSeverityCritical,
					Message:    "motor overheated",
					OccurredAt: sent.GetOccurredAt().AsTime(),
				},
			}
			if diff := cmp.Diff(want, robotFaults(t, robotID), equateTime); diff != "" {
				t.Errorf("faults mismatch (-want +got):\n%s", diff)
			}
		})
	})

	t.Run("A fault sent twice is stored once", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())
		sent := fault(robotID, "E_MOTOR")

		step(t, "When GW-1 reports the same fault twice", func(t *testing.T) {
			for range 2 {
				if err := gw.reportFault(t, sent); err != nil {
					t.Fatalf("report fault: %v", err)
				}
			}
		})
		step(t, "Then SN-1 has it once", func(t *testing.T) {
			if got := robotFaults(t, robotID); len(got) != 1 {
				t.Errorf("SN-1 has %d faults, want 1", len(got))
			}
		})
	})

	t.Run("A fault needs a code", func(t *testing.T) {
		gw, robotID := givenScannedRobot(t, newScenario())
		var err error

		step(t, "When GW-1 reports a fault without a code for SN-1", func(t *testing.T) {
			err = gw.reportFault(t, fault(robotID, ""))
		})
		step(t, "Then it is rejected with InvalidArgument", func(t *testing.T) {
			if code := status.Code(err); code != codes.InvalidArgument {
				t.Errorf("code = %s, want %s (error: %v)", code, codes.InvalidArgument, err)
			}
		})
		step(t, "And nothing is stored", func(t *testing.T) {
			if got := robotFaults(t, robotID); len(got) != 0 {
				t.Errorf("SN-1 has %d faults, want none", len(got))
			}
		})
	})
}

// equateTime compares times by the instant, whatever their location.
var equateTime = cmp.Comparer(func(a, b time.Time) bool { return a.Equal(b) })
