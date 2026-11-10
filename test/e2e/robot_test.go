//go:build e2e

package e2e

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
)

// TestRobotDiscovery: a gateway scans for its robots and reports the full
// list over gRPC. The system gives every new robot a robotId and keeps it
// across scans.
func TestRobotDiscovery(t *testing.T) {
	t.Run("Known robots keep their robotId", func(t *testing.T) {
		sc := newScenario()
		var (
			gw          gateway
			first, then map[string]string
		)

		step(t, "Given gateway GW-1 is registered", func(t *testing.T) {
			gw = registerGateway(t, sc.serial("GW-1"))
		})
		step(t, "And GW-1 scanned robot SN-1", func(t *testing.T) {
			first = gw.mustScan(t, sc.serial("SN-1"))
		})
		step(t, "When GW-1 scans robots SN-1 and SN-2", func(t *testing.T) {
			then = gw.mustScan(t, sc.serials("SN-1", "SN-2")...)
		})
		step(t, "Then SN-1 keeps its robotId", func(t *testing.T) {
			serial := sc.serial("SN-1")
			if then[serial] != first[serial] {
				t.Errorf("robotId = %s, want %s from the first scan", then[serial], first[serial])
			}
		})
		step(t, "And GW-1 knows robots SN-1 and SN-2", func(t *testing.T) {
			if diff := cmp.Diff(then, gw.knownRobots(t)); diff != "" {
				t.Errorf("known robots mismatch (-scanned +known):\n%s", diff)
			}
		})
	})

	t.Run("A scan without robots is accepted", func(t *testing.T) {
		sc := newScenario()
		var (
			gw  gateway
			err error
		)

		step(t, "Given gateway GW-1 is registered", func(t *testing.T) {
			gw = registerGateway(t, sc.serial("GW-1"))
		})
		step(t, "When GW-1 scans no robots", func(t *testing.T) {
			_, err = gw.scan(t)
		})
		step(t, "Then the scan succeeds", func(t *testing.T) {
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
		})
		step(t, "And GW-1 has no robots", func(t *testing.T) {
			if robots := gatewayRobots(t, gw.id); len(robots) != 0 {
				t.Errorf("gateway has %d robots, want none", len(robots))
			}
		})
	})

	t.Run("A scan is rejected", func(t *testing.T) {
		tests := []struct {
			name     string
			given    func(t *testing.T, sc scenario, gw gateway)
			scan     func(t *testing.T, sc scenario, gw gateway) error
			wantCode codes.Code
		}{
			{
				name: "with a duplicate serial number",
				scan: func(t *testing.T, sc scenario, gw gateway) error {
					_, err := gw.scan(t, sc.serials("SN-1", "SN-1")...)
					return err
				},
				wantCode: codes.InvalidArgument,
			},
			{
				name: "with a robot of another gateway",
				given: func(t *testing.T, sc scenario, _ gateway) {
					other := registerGateway(t, sc.serial("GW-2"))
					other.mustScan(t, sc.serial("SN-1"))
				},
				scan: func(t *testing.T, sc scenario, gw gateway) error {
					_, err := gw.scan(t, sc.serial("SN-1"))
					return err
				},
				wantCode: codes.PermissionDenied,
			},
			{
				name: "from a gateway that was never registered",
				scan: func(t *testing.T, sc scenario, _ gateway) error {
					_, err := gateway{id: uuid.New()}.scan(t, sc.serial("SN-1"))
					return err
				},
				wantCode: codes.PermissionDenied,
			},
			{
				name: "without a gatewayId",
				scan: func(t *testing.T, sc scenario, _ gateway) error {
					req := &gatewayv1.ReportScanRequest{SerialNumbers: []string{sc.serial("SN-1")}}
					_, err := gatewayService.ReportScan(t.Context(), req)
					return err
				},
				wantCode: codes.Unauthenticated,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sc := newScenario()
				var (
					gw  gateway
					err error
				)

				step(t, "Given gateway GW-1 is registered", func(t *testing.T) {
					gw = registerGateway(t, sc.serial("GW-1"))
					if tt.given != nil {
						tt.given(t, sc, gw)
					}
				})
				step(t, "When a scan is reported "+tt.name, func(t *testing.T) {
					err = tt.scan(t, sc, gw)
				})
				step(t, "Then it is rejected with "+tt.wantCode.String(), func(t *testing.T) {
					if code := status.Code(err); code != tt.wantCode {
						t.Errorf("code = %s, want %s (error: %v)", code, tt.wantCode, err)
					}
				})
			})
		}
	})
}
