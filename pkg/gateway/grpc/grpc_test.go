//go:build unit

package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	gatewaygrpc "github.com/umartahir93/olivaw-assignment/pkg/gateway/grpc"
	robotdomain "github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

var (
	registered = gatewaydomain.Gateway{
		ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		SerialNumber: "GW-1",
		Location:     "Munich",
		RegisteredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	robot1 = robotdomain.Robot{
		ID:           uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		SerialNumber: "SN-1",
		GatewayID:    registered.ID,
		FirstSeenAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	robot2 = robotdomain.Robot{
		ID:           uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		SerialNumber: "SN-2",
		GatewayID:    registered.ID,
		FirstSeenAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	errDatabase = errors.New("database down")
)

// gateways returns a gateway service that knows only the registered gateway.
func gateways() *gatewaygrpc.FakeGatewayService {
	return &gatewaygrpc.FakeGatewayService{
		Gateways: []gatewaydomain.Gateway{registered},
	}
}

// asGateway returns a context carrying the gatewayId metadata.
func asGateway(id string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(gatewaygrpc.GatewayIDKey, id))
}

func TestGatewayIDFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		gateways *gatewaygrpc.FakeGatewayService
		want     uuid.UUID
		wantCode codes.Code
	}{
		{
			name:     "registered gateway",
			ctx:      asGateway(registered.ID.String()),
			gateways: gateways(),
			want:     registered.ID,
			wantCode: codes.OK,
		},
		{
			name:     "no metadata",
			ctx:      context.Background(),
			gateways: gateways(),
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "gatewayId is not a UUID",
			ctx:      asGateway("gw-1"),
			gateways: gateways(),
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "gateway is not registered",
			ctx:      asGateway(uuid.NewString()),
			gateways: gateways(),
			wantCode: codes.PermissionDenied,
		},
		{
			name: "gateway service fails",
			ctx:  asGateway(registered.ID.String()),
			gateways: &gatewaygrpc.FakeGatewayService{
				Err: errDatabase,
			},
			// Returned as it is; the middleware logs it and answers INTERNAL.
			wantCode: codes.Unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gatewaygrpc.GatewayIDFromMetadata(tt.ctx, tt.gateways)

			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf("code = %s, want %s (error: %v)", code, tt.wantCode, err)
			}
			if got != tt.want {
				t.Errorf("gatewayId = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestListRobots(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		robots   *gatewaygrpc.FakeRobotService
		want     *gatewayv1.ListRobotsResponse
		wantCode codes.Code
	}{
		{
			name: "the gateway's robots",
			ctx:  asGateway(registered.ID.String()),
			robots: &gatewaygrpc.FakeRobotService{
				Robots: []robotdomain.Robot{robot1, robot2},
			},
			want: &gatewayv1.ListRobotsResponse{
				Robots: []*gatewayv1.Robot{
					{RobotId: robot1.ID.String(), SerialNumber: "SN-1"},
					{RobotId: robot2.ID.String(), SerialNumber: "SN-2"},
				},
			},
			wantCode: codes.OK,
		},
		{
			name:     "no robots",
			ctx:      asGateway(registered.ID.String()),
			robots:   &gatewaygrpc.FakeRobotService{},
			want:     &gatewayv1.ListRobotsResponse{},
			wantCode: codes.OK,
		},
		{
			name:     "gateway is not registered",
			ctx:      asGateway(uuid.NewString()),
			robots:   &gatewaygrpc.FakeRobotService{},
			wantCode: codes.PermissionDenied,
		},
		{
			name: "robot service fails",
			ctx:  asGateway(registered.ID.String()),
			robots: &gatewaygrpc.FakeRobotService{
				Err: errDatabase,
			},
			// Returned as it is; the middleware logs it and answers INTERNAL.
			wantCode: codes.Unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gatewaygrpc.NewServer(gateways(), tt.robots).ListRobots(tt.ctx, &gatewayv1.ListRobotsRequest{})

			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf("code = %s, want %s (error: %v)", code, tt.wantCode, err)
			}
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReportScan(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		request     *gatewayv1.ReportScanRequest
		robots      *gatewaygrpc.FakeRobotService
		want        *gatewayv1.ReportScanResponse
		wantCode    codes.Code
		wantScanned []robotdomain.Robot
	}{
		{
			name: "scanned robots come back with their IDs",
			ctx:  asGateway(registered.ID.String()),
			request: &gatewayv1.ReportScanRequest{
				SerialNumbers: []string{"SN-1", "SN-2"},
			},
			robots: &gatewaygrpc.FakeRobotService{
				Robots: []robotdomain.Robot{robot1, robot2},
			},
			want: &gatewayv1.ReportScanResponse{
				Robots: []*gatewayv1.Robot{
					{RobotId: robot1.ID.String(), SerialNumber: "SN-1"},
					{RobotId: robot2.ID.String(), SerialNumber: "SN-2"},
				},
			},
			wantCode: codes.OK,
			wantScanned: []robotdomain.Robot{
				{SerialNumber: "SN-1", GatewayID: registered.ID},
				{SerialNumber: "SN-2", GatewayID: registered.ID},
			},
		},
		{
			name: "empty scan",
			ctx:  asGateway(registered.ID.String()),
			request: &gatewayv1.ReportScanRequest{
				SerialNumbers: []string{},
			},
			robots:      &gatewaygrpc.FakeRobotService{},
			want:        &gatewayv1.ReportScanResponse{},
			wantCode:    codes.OK,
			wantScanned: []robotdomain.Robot{},
		},
		{
			name: "empty serial number",
			ctx:  asGateway(registered.ID.String()),
			request: &gatewayv1.ReportScanRequest{
				SerialNumbers: []string{"SN-1", ""},
			},
			robots:   &gatewaygrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "duplicate serial number",
			ctx:  asGateway(registered.ID.String()),
			request: &gatewayv1.ReportScanRequest{
				SerialNumbers: []string{"SN-1", "SN-1"},
			},
			robots:   &gatewaygrpc.FakeRobotService{},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "no gatewayId",
			ctx:  context.Background(),
			request: &gatewayv1.ReportScanRequest{
				SerialNumbers: []string{"SN-1"},
			},
			robots:   &gatewaygrpc.FakeRobotService{},
			wantCode: codes.Unauthenticated,
		},
		{
			name: "robot of another gateway",
			ctx:  asGateway(registered.ID.String()),
			request: &gatewayv1.ReportScanRequest{
				SerialNumbers: []string{"SN-9"},
			},
			robots: &gatewaygrpc.FakeRobotService{
				Err: robotdomain.ErrOtherGateway,
			},
			wantCode: codes.PermissionDenied,
			wantScanned: []robotdomain.Robot{
				{SerialNumber: "SN-9", GatewayID: registered.ID},
			},
		},
		{
			name: "robot service fails",
			ctx:  asGateway(registered.ID.String()),
			request: &gatewayv1.ReportScanRequest{
				SerialNumbers: []string{"SN-1"},
			},
			robots: &gatewaygrpc.FakeRobotService{
				Err: errDatabase,
			},
			// Returned as it is; the middleware logs it and answers INTERNAL.
			wantCode: codes.Unknown,
			wantScanned: []robotdomain.Robot{
				{SerialNumber: "SN-1", GatewayID: registered.ID},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gatewaygrpc.NewServer(gateways(), tt.robots).ReportScan(tt.ctx, tt.request)

			if code := status.Code(err); code != tt.wantCode {
				t.Fatalf("code = %s, want %s (error: %v)", code, tt.wantCode, err)
			}
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
			// The handler gives every scanned robot a random ID, so IDs are
			// checked separately.
			if diff := cmp.Diff(tt.wantScanned, tt.robots.Scanned, cmpopts.IgnoreFields(robotdomain.Robot{}, "ID")); diff != "" {
				t.Errorf("scanned robots mismatch (-want +got):\n%s", diff)
			}
			for _, r := range tt.robots.Scanned {
				if r.ID == uuid.Nil {
					t.Errorf("robot %s: no ID", r.SerialNumber)
				}
			}
		})
	}
}
