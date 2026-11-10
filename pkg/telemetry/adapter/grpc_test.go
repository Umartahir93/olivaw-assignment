//go:build unit

package adapter

import (
	"context"
	"testing"

	"github.com/google/uuid"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
)

func TestListRobotsReturnsResponse(t *testing.T) {
	resp, err := (&GatewayServer{}).ListRobots(context.Background(), &gatewayv1.ListRobotsRequest{})
	if err != nil || resp == nil {
		t.Fatalf("ListRobots: resp %v, err %v", resp, err)
	}
}

func TestReportScanAssignsRobotIDs(t *testing.T) {
	req := &gatewayv1.ReportScanRequest{SerialNumbers: []string{"SN-1", "SN-2"}}

	resp, err := (&GatewayServer{}).ReportScan(context.Background(), req)
	if err != nil {
		t.Fatalf("ReportScan: %v", err)
	}
	if len(resp.GetRobots()) != 2 {
		t.Fatalf("got %d robots, want 2", len(resp.GetRobots()))
	}
	for i, robot := range resp.GetRobots() {
		if robot.GetSerialNumber() != req.GetSerialNumbers()[i] {
			t.Errorf("robot %d: serial %q, want %q", i, robot.GetSerialNumber(), req.GetSerialNumbers()[i])
		}
		if _, err := uuid.Parse(robot.GetRobotId()); err != nil {
			t.Errorf("robot %d: robotId %q is not a UUID", i, robot.GetRobotId())
		}
	}
}

func TestReportMeasurementReturnsResponse(t *testing.T) {
	resp, err := (&RobotServer{}).ReportMeasurement(context.Background(), &gatewayv1.ReportMeasurementRequest{})
	if err != nil || resp == nil {
		t.Fatalf("ReportMeasurement: resp %v, err %v", resp, err)
	}
}

func TestReportFaultReturnsResponse(t *testing.T) {
	resp, err := (&RobotServer{}).ReportFault(context.Background(), &gatewayv1.ReportFaultRequest{})
	if err != nil || resp == nil {
		t.Fatalf("ReportFault: resp %v, err %v", resp, err)
	}
}
