package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
)

func TestHandlersReturnSuccess(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()
	id := uuid.New()

	tests := []struct {
		name string
		call func() (any, error)
		want any
	}{
		{"create gateway", func() (any, error) {
			return h.CreateGateway(ctx, &fleetv1.GatewayCreate{SerialNumber: "GW-1", Location: "HQ"})
		}, &fleetv1.GatewayHeaders{}},
		{"list gateways", func() (any, error) { return h.ListGateways(ctx, fleetv1.ListGatewaysParams{}) }, &fleetv1.GatewayPage{}},
		{"get gateway", func() (any, error) { return h.GetGateway(ctx, fleetv1.GetGatewayParams{GatewayId: id}) }, &fleetv1.Gateway{}},
		{"list gateway robots", func() (any, error) {
			return h.ListGatewayRobots(ctx, fleetv1.ListGatewayRobotsParams{GatewayId: id})
		}, &fleetv1.RobotPage{}},
		{"get robot", func() (any, error) { return h.GetRobot(ctx, fleetv1.GetRobotParams{RobotId: id}) }, &fleetv1.Robot{}},
		{"list measurements", func() (any, error) {
			return h.ListRobotMeasurements(ctx, fleetv1.ListRobotMeasurementsParams{RobotId: id})
		}, &fleetv1.MeasurementPage{}},
		{"list faults", func() (any, error) {
			return h.ListRobotFaults(ctx, fleetv1.ListRobotFaultsParams{RobotId: id})
		}, &fleetv1.FaultPage{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.call()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got, want := fmt.Sprintf("%T", res), fmt.Sprintf("%T", tt.want); got != want {
				t.Fatalf("returned %s, want %s", got, want)
			}
		})
	}
}
