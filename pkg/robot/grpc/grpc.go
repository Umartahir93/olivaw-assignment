// Package grpc implements the RobotService of the gRPC contract: data about
// one robot, sent by its gateway, which the system stores.
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
	ReportMeasurement(ctx context.Context, gatewayID uuid.UUID, m domain.Measurement) error
	ReportFault(ctx context.Context, gatewayID uuid.UUID, f domain.Fault) error
}

// Server implements gatewayv1.RobotServiceServer: it validates measurements
// and faults and stores them through the robot service.
type Server struct {
	gatewayv1.UnimplementedRobotServiceServer

	gateways gatewaygrpc.GatewayService
	robots   RobotService
}

// NewServer returns a Server backed by the gateway and robot services.
func NewServer(gateways gatewaygrpc.GatewayService, robots RobotService) *Server {
	return &Server{gateways: gateways, robots: robots}
}

// ReportMeasurement stores one measurement of one robot of the calling
// gateway.
func (s *Server) ReportMeasurement(ctx context.Context, req *gatewayv1.ReportMeasurementRequest) (*gatewayv1.ReportMeasurementResponse, error) {
	robotID, err := validateMeasurement(req.GetMeasurement())
	if err != nil {
		return nil, err
	}
	gatewayID, err := gatewaygrpc.GatewayIDFromMetadata(ctx, s.gateways)
	if err != nil {
		return nil, err
	}
	if err = s.robots.ReportMeasurement(ctx, gatewayID, measurementToDomain(robotID, req.GetMeasurement())); err != nil {
		return nil, toStatus(err)
	}
	return &gatewayv1.ReportMeasurementResponse{}, nil
}

// ReportFault stores one fault of one robot of the calling gateway.
func (s *Server) ReportFault(ctx context.Context, req *gatewayv1.ReportFaultRequest) (*gatewayv1.ReportFaultResponse, error) {
	robotID, err := validateFault(req.GetFault())
	if err != nil {
		return nil, err
	}
	gatewayID, err := gatewaygrpc.GatewayIDFromMetadata(ctx, s.gateways)
	if err != nil {
		return nil, err
	}
	if err = s.robots.ReportFault(ctx, gatewayID, faultToDomain(robotID, req.GetFault())); err != nil {
		return nil, toStatus(err)
	}
	return &gatewayv1.ReportFaultResponse{}, nil
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

// measurementToDomain turns a validated measurement into a domain one.
func measurementToDomain(robotID uuid.UUID, m *gatewayv1.Measurement) domain.Measurement {
	out := domain.Measurement{
		RobotID:          robotID,
		BatteryPct:       m.BatteryPct,
		Activity:         activities[m.GetActivity()],
		ActiveFaultCodes: m.GetActiveFaultCodes(),
		MeasuredAt:       m.GetMeasuredAt().AsTime(),
	}
	if out.Activity == "" {
		out.Activity = domain.ActivityUnknown // a value newer than this server
	}
	if p := m.GetPosition(); p != nil {
		out.Position = &domain.Position{MapID: p.GetMapId(), X: p.GetX(), Y: p.GetY(), HeadingRad: p.GetHeadingRad()}
	}
	return out
}

// faultToDomain turns a validated fault into a domain one.
func faultToDomain(robotID uuid.UUID, f *gatewayv1.Fault) domain.Fault {
	severity, ok := severities[f.GetSeverity()]
	if !ok {
		severity = domain.SeverityUnknown // a value newer than this server
	}
	return domain.Fault{
		RobotID:    robotID,
		Code:       f.GetCode(),
		Severity:   severity,
		Message:    f.GetMessage(),
		OccurredAt: f.GetOccurredAt().AsTime(),
	}
}

var (
	activities = map[gatewayv1.Activity]domain.Activity{
		gatewayv1.Activity_ACTIVITY_UNSPECIFIED: domain.ActivityUnknown,
		gatewayv1.Activity_ACTIVITY_IDLE:        domain.ActivityIdle,
		gatewayv1.Activity_ACTIVITY_MOVING:      domain.ActivityMoving,
		gatewayv1.Activity_ACTIVITY_CHARGING:    domain.ActivityCharging,
		gatewayv1.Activity_ACTIVITY_ERROR:       domain.ActivityError,
	}
	severities = map[gatewayv1.Fault_Severity]domain.Severity{
		gatewayv1.Fault_SEVERITY_UNSPECIFIED: domain.SeverityUnknown,
		gatewayv1.Fault_SEVERITY_WARNING:     domain.SeverityWarning,
		gatewayv1.Fault_SEVERITY_CRITICAL:    domain.SeverityCritical,
	}
)

func finite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// toStatus maps an error from the robot service to the contract's codes.
// Other errors are unexpected and returned as they are; the middleware logs
// them and answers INTERNAL.
func toStatus(err error) error {
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrOtherGateway) {
		return status.Error(codes.PermissionDenied, "robot is not one of this gateway's robots")
	}
	return err
}
