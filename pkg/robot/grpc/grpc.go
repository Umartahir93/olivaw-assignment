// Package grpc implements the RobotService of the gRPC contract: data about
// one robot, sent by its gateway.
package grpc

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	gatewaygrpc "github.com/umartahir93/olivaw-assignment/pkg/gateway/grpc"
	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// RobotService is what the RobotService handlers need from the robot service.
type RobotService interface {
	MarkSeen(ctx context.Context, gatewayID, robotID uuid.UUID) (domain.Robot, error)
}

// Server implements gatewayv1.RobotServiceServer. Measurements and faults are
// validated and the robot is marked as seen; storing them comes with the
// measurement pipeline.
type Server struct {
	gatewayv1.UnimplementedRobotServiceServer

	gateways gatewaygrpc.GatewayService
	robots   RobotService
}

// NewServer returns a Server backed by the gateway and robot services.
func NewServer(gateways gatewaygrpc.GatewayService, robots RobotService) *Server {
	return &Server{gateways: gateways, robots: robots}
}

// ReportMeasurement accepts one measurement of one robot of the calling
// gateway.
func (s *Server) ReportMeasurement(ctx context.Context, req *gatewayv1.ReportMeasurementRequest) (*gatewayv1.ReportMeasurementResponse, error) {
	robotID, err := validateMeasurement(req.GetMeasurement())
	if err != nil {
		return nil, err
	}
	if err := s.markSeen(ctx, robotID); err != nil {
		return nil, err
	}
	return &gatewayv1.ReportMeasurementResponse{}, nil
}

// ReportFault accepts one fault of one robot of the calling gateway.
func (s *Server) ReportFault(ctx context.Context, req *gatewayv1.ReportFaultRequest) (*gatewayv1.ReportFaultResponse, error) {
	robotID, err := validateFault(req.GetFault())
	if err != nil {
		return nil, err
	}
	if err := s.markSeen(ctx, robotID); err != nil {
		return nil, err
	}
	return &gatewayv1.ReportFaultResponse{}, nil
}

// markSeen checks that the robot belongs to the calling gateway and records
// that it was heard from.
func (s *Server) markSeen(ctx context.Context, robotID uuid.UUID) error {
	gatewayID, err := gatewaygrpc.CallingGateway(ctx, s.gateways)
	if err != nil {
		return err
	}
	if _, err = s.robots.MarkSeen(ctx, gatewayID, robotID); err != nil {
		return toStatus(err)
	}
	return nil
}

func validateMeasurement(m *gatewayv1.Measurement) (uuid.UUID, error) {
	if m == nil {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "measurement is required")
	}
	robotID, err := uuid.Parse(m.GetRobotId())
	if err != nil {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "robot_id must be a UUID")
	}
	if m.GetMeasuredAt() == nil || m.GetMeasuredAt().CheckValid() != nil {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "measured_at must be a valid timestamp")
	}
	if m.BatteryPct != nil {
		pct := m.GetBatteryPct()
		if !finite(pct) || pct < 0 || pct > 100 {
			return uuid.UUID{}, status.Error(codes.InvalidArgument, "battery_pct must be between 0 and 100")
		}
	}
	if p := m.GetPosition(); p != nil {
		if !finite(p.GetX()) || !finite(p.GetY()) || !finite(p.GetHeadingRad()) {
			return uuid.UUID{}, status.Error(codes.InvalidArgument, "position values must be finite")
		}
	}
	return robotID, nil
}

func validateFault(f *gatewayv1.Fault) (uuid.UUID, error) {
	if f == nil {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "fault is required")
	}
	robotID, err := uuid.Parse(f.GetRobotId())
	if err != nil {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "robot_id must be a UUID")
	}
	if f.GetCode() == "" {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "code is required")
	}
	if f.GetOccurredAt() == nil || f.GetOccurredAt().CheckValid() != nil {
		return uuid.UUID{}, status.Error(codes.InvalidArgument, "occurred_at must be a valid timestamp")
	}
	return robotID, nil
}

func finite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// toStatus maps an error from the robot service to the contract's codes.
func toStatus(err error) error {
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrOtherGateway) {
		return status.Error(codes.PermissionDenied, "robot is not one of this gateway's robots")
	}
	return status.Error(codes.Internal, "internal error")
}
