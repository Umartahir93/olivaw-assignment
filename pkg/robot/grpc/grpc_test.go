//go:build unit

package grpc_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	gatewaygrpc "github.com/umartahir93/olivaw-assignment/pkg/gateway/grpc"
	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
	robotgrpc "github.com/umartahir93/olivaw-assignment/pkg/robot/grpc"
)

var (
	registered = gatewaydomain.Gateway{
		ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		SerialNumber: "GW-1",
		Location:     "Munich",
		RegisteredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	robotID     = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	measuredAt  = timestamppb.New(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	errDatabase = errors.New("database down")
)

// newServer returns a Server whose gateway service knows only the
// registered gateway.
func newServer(robots *robotgrpc.FakeRobotService) *robotgrpc.Server {
	gateways := &gatewaygrpc.FakeGatewayService{
		Gateways: []gatewaydomain.Gateway{registered},
	}
	return robotgrpc.NewServer(gateways, robots)
}

// asGateway returns a context carrying the gatewayId metadata.
func asGateway(id string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(gatewaygrpc.GatewayIDKey, id))
}

func TestReportMeasurement(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		measurement *gatewayv1.Measurement
		robots      *robotgrpc.FakeRobotService
		wantCode    codes.Code
		wantSeen    []uuid.UUID
	}{
		{
			name: "valid measurement marks the robot seen",
			ctx:  asGateway(registered.ID.String()),
			measurement: &gatewayv1.Measurement{
				RobotId:    robotID.String(),
				BatteryPct: proto.Float64(80),
				Position:   &gatewayv1.Position{MapId: "floor-1", X: 1, Y: 2, HeadingRad: 0.5},
				MeasuredAt: measuredAt,
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.OK,
			wantSeen: []uuid.UUID{robotID},
		},
		{
			name:        "measurement missing",
			ctx:         asGateway(registered.ID.String()),
			measurement: nil,
			robots:      &robotgrpc.FakeRobotService{},
			wantCode:    codes.InvalidArgument,
		},
		{
			name: "robot_id is not a UUID",
			ctx:  asGateway(registered.ID.String()),
			measurement: &gatewayv1.Measurement{
				RobotId:    "robot-1",
				MeasuredAt: measuredAt,
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "measured_at missing",
			ctx:  asGateway(registered.ID.String()),
			measurement: &gatewayv1.Measurement{
				RobotId: robotID.String(),
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "battery above 100",
			ctx:  asGateway(registered.ID.String()),
			measurement: &gatewayv1.Measurement{
				RobotId:    robotID.String(),
				BatteryPct: proto.Float64(101),
				MeasuredAt: measuredAt,
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "position not finite",
			ctx:  asGateway(registered.ID.String()),
			measurement: &gatewayv1.Measurement{
				RobotId:    robotID.String(),
				Position:   &gatewayv1.Position{X: math.NaN()},
				MeasuredAt: measuredAt,
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "no gatewayId",
			ctx:  context.Background(),
			measurement: &gatewayv1.Measurement{
				RobotId:    robotID.String(),
				MeasuredAt: measuredAt,
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.Unauthenticated,
		},
		{
			name: "gateway is not registered",
			ctx:  asGateway(uuid.NewString()),
			measurement: &gatewayv1.Measurement{
				RobotId:    robotID.String(),
				MeasuredAt: measuredAt,
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.PermissionDenied,
		},
		{
			name: "unknown robot",
			ctx:  asGateway(registered.ID.String()),
			measurement: &gatewayv1.Measurement{
				RobotId:    robotID.String(),
				MeasuredAt: measuredAt,
			},
			robots: &robotgrpc.FakeRobotService{
				Err: domain.ErrNotFound,
			},
			wantCode: codes.PermissionDenied,
		},
		{
			name: "robot of another gateway",
			ctx:  asGateway(registered.ID.String()),
			measurement: &gatewayv1.Measurement{
				RobotId:    robotID.String(),
				MeasuredAt: measuredAt,
			},
			robots: &robotgrpc.FakeRobotService{
				Err: domain.ErrOtherGateway,
			},
			wantCode: codes.PermissionDenied,
		},
		{
			name: "robot service fails",
			ctx:  asGateway(registered.ID.String()),
			measurement: &gatewayv1.Measurement{
				RobotId:    robotID.String(),
				MeasuredAt: measuredAt,
			},
			robots: &robotgrpc.FakeRobotService{
				Err: errDatabase,
			},
			wantCode: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newServer(tt.robots).ReportMeasurement(tt.ctx, &gatewayv1.ReportMeasurementRequest{Measurement: tt.measurement})

			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf("code = %s, want %s (error: %v)", code, tt.wantCode, err)
			}
			if diff := cmp.Diff(tt.wantSeen, tt.robots.Seen); diff != "" {
				t.Errorf("robots marked seen mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReportFault(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		fault    *gatewayv1.Fault
		robots   *robotgrpc.FakeRobotService
		wantCode codes.Code
		wantSeen []uuid.UUID
	}{
		{
			name: "valid fault marks the robot seen",
			ctx:  asGateway(registered.ID.String()),
			fault: &gatewayv1.Fault{
				RobotId:    robotID.String(),
				Code:       "E_MOTOR",
				Severity:   gatewayv1.Fault_SEVERITY_CRITICAL,
				OccurredAt: measuredAt,
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.OK,
			wantSeen: []uuid.UUID{robotID},
		},
		{
			name:     "fault missing",
			ctx:      asGateway(registered.ID.String()),
			fault:    nil,
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "code empty",
			ctx:  asGateway(registered.ID.String()),
			fault: &gatewayv1.Fault{
				RobotId:    robotID.String(),
				OccurredAt: measuredAt,
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "occurred_at missing",
			ctx:  asGateway(registered.ID.String()),
			fault: &gatewayv1.Fault{
				RobotId: robotID.String(),
				Code:    "E_MOTOR",
			},
			robots:   &robotgrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "robot of another gateway",
			ctx:  asGateway(registered.ID.String()),
			fault: &gatewayv1.Fault{
				RobotId:    robotID.String(),
				Code:       "E_MOTOR",
				OccurredAt: measuredAt,
			},
			robots: &robotgrpc.FakeRobotService{
				Err: domain.ErrOtherGateway,
			},
			wantCode: codes.PermissionDenied,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newServer(tt.robots).ReportFault(tt.ctx, &gatewayv1.ReportFaultRequest{Fault: tt.fault})

			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf("code = %s, want %s (error: %v)", code, tt.wantCode, err)
			}
			if diff := cmp.Diff(tt.wantSeen, tt.robots.Seen); diff != "" {
				t.Errorf("robots marked seen mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
