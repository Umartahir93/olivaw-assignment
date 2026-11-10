//go:build unit

package api_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/api"
	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	"github.com/umartahir93/olivaw-assignment/pkg/gateway"
	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	"github.com/umartahir93/olivaw-assignment/pkg/robot"
	robotdomain "github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

var (
	registered = gatewaydomain.Gateway{
		ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		SerialNumber: "GW-1",
		Location:     "Munich",
		RegisteredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	knownRobot = robotdomain.Robot{
		ID:           uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		SerialNumber: "SN-1",
		GatewayID:    registered.ID,
		FirstSeenAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
)

// TestHandlers checks which response each handler picks. The response
// bodies are built by gatewayToREST and robotToREST.
func TestHandlers(t *testing.T) {
	h := api.NewHandler(
		gateway.NewService(&gateway.FakeRepository{Gateways: []gatewaydomain.Gateway{registered}}),
		robot.NewService(&robot.FakeRepository{Robots: []robotdomain.Robot{knownRobot}}),
	)
	ctx := context.Background()
	unknown := uuid.New()

	tests := []struct {
		name string
		call func() (any, error)
		want any
	}{
		{
			name: "create gateway",
			call: func() (any, error) {
				return h.CreateGateway(ctx, &fleetv1.GatewayCreate{SerialNumber: "GW-NEW", Location: "Berlin"})
			},
			want: &fleetv1.GatewayHeaders{},
		},
		{
			name: "create gateway with a taken serial number",
			call: func() (any, error) {
				return h.CreateGateway(ctx, &fleetv1.GatewayCreate{SerialNumber: "GW-1", Location: "Berlin"})
			},
			want: &fleetv1.CreateGatewayConflict{},
		},
		{
			name: "list gateways",
			call: func() (any, error) { return h.ListGateways(ctx) },
			want: &fleetv1.GatewayList{},
		},
		{
			name: "get gateway",
			call: func() (any, error) { return h.GetGateway(ctx, fleetv1.GetGatewayParams{GatewayId: registered.ID}) },
			want: &fleetv1.Gateway{},
		},
		{
			name: "get unknown gateway",
			call: func() (any, error) { return h.GetGateway(ctx, fleetv1.GetGatewayParams{GatewayId: unknown}) },
			want: &fleetv1.GetGatewayNotFound{},
		},
		{
			name: "list gateway robots",
			call: func() (any, error) {
				return h.ListGatewayRobots(ctx, fleetv1.ListGatewayRobotsParams{GatewayId: registered.ID})
			},
			want: &fleetv1.RobotList{},
		},
		{
			name: "list robots of unknown gateway",
			call: func() (any, error) {
				return h.ListGatewayRobots(ctx, fleetv1.ListGatewayRobotsParams{GatewayId: unknown})
			},
			want: &fleetv1.ListGatewayRobotsNotFound{},
		},
		{
			name: "get robot",
			call: func() (any, error) { return h.GetRobot(ctx, fleetv1.GetRobotParams{RobotId: knownRobot.ID}) },
			want: &fleetv1.Robot{},
		},
		{
			name: "get unknown robot",
			call: func() (any, error) { return h.GetRobot(ctx, fleetv1.GetRobotParams{RobotId: unknown}) },
			want: &fleetv1.GetRobotNotFound{},
		},
		{
			name: "list measurements",
			call: func() (any, error) {
				return h.ListRobotMeasurements(ctx, fleetv1.ListRobotMeasurementsParams{RobotId: knownRobot.ID})
			},
			want: &fleetv1.MeasurementList{},
		},
		{
			name: "list faults of unknown robot",
			call: func() (any, error) {
				return h.ListRobotFaults(ctx, fleetv1.ListRobotFaultsParams{RobotId: unknown})
			},
			want: &fleetv1.ListRobotFaultsNotFound{},
		},
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
