package api

import (
	"context"
	"testing"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
)

func TestHandlersReturnSuccess(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	if res, err := h.ListRobots(ctx, fleetv1.ListRobotsParams{}); err != nil {
		t.Errorf("ListRobots: %v", err)
	} else if _, ok := res.(*fleetv1.RobotPage); !ok {
		t.Errorf("ListRobots returned %T, want *RobotPage", res)
	}

	if res, err := h.CreateRobot(ctx, &fleetv1.RobotCreate{SerialNumber: "SN-1", Location: "HQ"}); err != nil {
		t.Errorf("CreateRobot: %v", err)
	} else if _, ok := res.(*fleetv1.RobotHeaders); !ok {
		t.Errorf("CreateRobot returned %T, want *RobotHeaders", res)
	}

	if res, err := h.GetRobot(ctx, fleetv1.GetRobotParams{}); err != nil {
		t.Errorf("GetRobot: %v", err)
	} else if _, ok := res.(*fleetv1.Robot); !ok {
		t.Errorf("GetRobot returned %T, want *Robot", res)
	}

	if res, err := h.DecommissionRobot(ctx, fleetv1.DecommissionRobotParams{}); err != nil {
		t.Errorf("DecommissionRobot: %v", err)
	} else if _, ok := res.(*fleetv1.DecommissionRobotNoContent); !ok {
		t.Errorf("DecommissionRobot returned %T, want *DecommissionRobotNoContent", res)
	}

	if res, err := h.ListRobotTelemetry(ctx, fleetv1.ListRobotTelemetryParams{}); err != nil {
		t.Errorf("ListRobotTelemetry: %v", err)
	} else if _, ok := res.(*fleetv1.TelemetryPage); !ok {
		t.Errorf("ListRobotTelemetry returned %T, want *TelemetryPage", res)
	}

	if res, err := h.ListRobotFaults(ctx, fleetv1.ListRobotFaultsParams{}); err != nil {
		t.Errorf("ListRobotFaults: %v", err)
	} else if _, ok := res.(*fleetv1.FaultPage); !ok {
		t.Errorf("ListRobotFaults returned %T, want *FaultPage", res)
	}
}
