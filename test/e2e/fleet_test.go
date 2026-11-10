//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	gatewaygrpc "github.com/umartahir93/olivaw-assignment/pkg/gateway/grpc"
)

// step runs one Given / When / Then step as a subtest. Later steps build on
// earlier ones, so a failed step ends the scenario.
func step(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.FailNow()
	}
}

// scenario keeps the serial numbers of one scenario unique, so scenarios
// neither depend on each other nor on what the database already holds.
type scenario struct {
	suffix string
}

func newScenario() scenario {
	return scenario{suffix: uuid.NewString()[:8]}
}

// serial returns the scenario's serial number for name, e.g. "SN-1-1a2b3c4d".
func (s scenario) serial(name string) string {
	return name + "-" + s.suffix
}

// serials returns the scenario's serial numbers for names.
func (s scenario) serials(names ...string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = s.serial(name)
	}
	return out
}

// gateway is a registered gateway; its calls carry its gatewayId.
type gateway struct {
	id uuid.UUID
}

// asGateway returns a context for gRPC calls made by the gateway with id.
func asGateway(t *testing.T, id string) context.Context {
	return metadata.AppendToOutgoingContext(t.Context(), gatewaygrpc.GatewayIDKey, id)
}

// --- REST, as the operator and the fleet manager ---

// register registers a gateway and returns the response, whatever it is.
func register(t *testing.T, serial, location string, name *string) fleetv1.CreateGatewayRes {
	t.Helper()
	req := &fleetv1.GatewayCreate{SerialNumber: serial, Location: location}
	if name != nil {
		req.Name = fleetv1.NewOptString(*name)
	}
	res, err := rest.CreateGateway(t.Context(), req)
	if err != nil {
		t.Fatalf("register gateway %s: %v", serial, err)
	}
	return res
}

// registerGateway registers a gateway and fails the test unless it is created.
func registerGateway(t *testing.T, serial string) gateway {
	t.Helper()
	created, ok := register(t, serial, "Munich HQ", nil).(*fleetv1.GatewayHeaders)
	if !ok {
		t.Fatalf("register gateway %s: not created", serial)
	}
	return gateway{id: created.Response.GatewayId}
}

// postRaw posts body as JSON without the generated client, which would
// refuse to send a body that breaks the contract. It returns the status.
func postRaw(t *testing.T, path string, body any) int {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, restURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

// getGateway returns the gateway, or fails the test if it is not found.
func getGateway(t *testing.T, id uuid.UUID) *fleetv1.Gateway {
	t.Helper()
	res, err := rest.GetGateway(t.Context(), fleetv1.GetGatewayParams{GatewayId: id})
	if err != nil {
		t.Fatalf("get gateway %s: %v", id, err)
	}
	g, ok := res.(*fleetv1.Gateway)
	if !ok {
		t.Fatalf("get gateway %s: got %T", id, res)
	}
	return g
}

// gatewayRobots returns the gateway's robots by serial number.
func gatewayRobots(t *testing.T, id uuid.UUID) map[string]fleetv1.Robot {
	t.Helper()
	res, err := rest.ListGatewayRobots(t.Context(), fleetv1.ListGatewayRobotsParams{GatewayId: id})
	if err != nil {
		t.Fatalf("list robots of gateway %s: %v", id, err)
	}
	list, ok := res.(*fleetv1.RobotList)
	if !ok {
		t.Fatalf("list robots of gateway %s: got %T", id, res)
	}
	robots := make(map[string]fleetv1.Robot, len(list.Items))
	for _, r := range list.Items {
		robots[r.SerialNumber] = r
	}
	return robots
}

// getRobot returns the robot, or fails the test if it is not found.
func getRobot(t *testing.T, id string) *fleetv1.Robot {
	t.Helper()
	res, err := rest.GetRobot(t.Context(), fleetv1.GetRobotParams{RobotId: uuid.MustParse(id)})
	if err != nil {
		t.Fatalf("get robot %s: %v", id, err)
	}
	r, ok := res.(*fleetv1.Robot)
	if !ok {
		t.Fatalf("get robot %s: got %T", id, res)
	}
	return r
}

// --- gRPC, as the gateway ---

// scan reports the serial numbers as the gateway's robot list and returns
// the robotIds by serial number.
func (g gateway) scan(t *testing.T, serials ...string) (map[string]string, error) {
	t.Helper()
	res, err := gatewayService.ReportScan(asGateway(t, g.id.String()), &gatewayv1.ReportScanRequest{SerialNumbers: serials})
	if err != nil {
		return nil, err
	}
	return robotIDs(res.GetRobots()), nil
}

// mustScan is scan for Given steps: it fails the test on any error.
func (g gateway) mustScan(t *testing.T, serials ...string) map[string]string {
	t.Helper()
	ids, err := g.scan(t, serials...)
	if err != nil {
		t.Fatalf("scan %v: %v", serials, err)
	}
	return ids
}

// knownRobots asks the system over gRPC which robots it knows for the
// gateway, by serial number.
func (g gateway) knownRobots(t *testing.T) map[string]string {
	t.Helper()
	res, err := gatewayService.ListRobots(asGateway(t, g.id.String()), &gatewayv1.ListRobotsRequest{})
	if err != nil {
		t.Fatalf("list robots: %v", err)
	}
	return robotIDs(res.GetRobots())
}

// reportMeasurement sends one measurement as the gateway.
func (g gateway) reportMeasurement(t *testing.T, m *gatewayv1.Measurement) error {
	t.Helper()
	_, err := robotService.ReportMeasurement(asGateway(t, g.id.String()), &gatewayv1.ReportMeasurementRequest{Measurement: m})
	return err
}

// reportFault sends one fault as the gateway.
func (g gateway) reportFault(t *testing.T, robotID, code string) error {
	t.Helper()
	_, err := robotService.ReportFault(asGateway(t, g.id.String()), &gatewayv1.ReportFaultRequest{Fault: &gatewayv1.Fault{
		RobotId:    robotID,
		Code:       code,
		Severity:   gatewayv1.Fault_SEVERITY_CRITICAL,
		Message:    "motor overheated",
		OccurredAt: timestamppb.Now(),
	}})
	return err
}

// measurement returns a valid measurement of the robot.
func measurement(robotID string, batteryPct float64) *gatewayv1.Measurement {
	return &gatewayv1.Measurement{
		RobotId:    robotID,
		BatteryPct: proto.Float64(batteryPct),
		Position:   &gatewayv1.Position{MapId: "floor-1", X: 12.5, Y: 3, HeadingRad: 1.57},
		Activity:   gatewayv1.Activity_ACTIVITY_MOVING,
		MeasuredAt: timestamppb.New(time.Now()),
	}
}

func robotIDs(robots []*gatewayv1.Robot) map[string]string {
	ids := make(map[string]string, len(robots))
	for _, r := range robots {
		ids[r.GetSerialNumber()] = r.GetRobotId()
	}
	return ids
}
