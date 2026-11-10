// Package grpc implements the GatewayService of the gRPC contract: the calls
// a gateway makes about its robot list.
package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	robotdomain "github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// GatewayIDKey is the metadata key a gateway sends its gatewayId in.
const GatewayIDKey = "x-gateway-id"

// GatewayService is what the handlers need from the gateway service.
type GatewayService interface {
	MarkSeen(ctx context.Context, id uuid.UUID) (gatewaydomain.Gateway, error)
}

// RobotService is what the handlers need from the robot service.
type RobotService interface {
	GatewayRobots(ctx context.Context, gatewayID uuid.UUID) ([]robotdomain.Robot, error)
	ReportScan(ctx context.Context, scanned []robotdomain.Robot) ([]robotdomain.Robot, error)
}

// Server implements gatewayv1.GatewayServiceServer.
type Server struct {
	gatewayv1.UnimplementedGatewayServiceServer

	gateways GatewayService
	robots   RobotService
}

// NewServer returns a Server backed by the gateway and robot services.
func NewServer(gateways GatewayService, robots RobotService) *Server {
	return &Server{gateways: gateways, robots: robots}
}

// ListRobots returns the robots the system knows for the calling gateway.
func (s *Server) ListRobots(ctx context.Context, _ *gatewayv1.ListRobotsRequest) (*gatewayv1.ListRobotsResponse, error) {
	gatewayID, err := CallingGateway(ctx, s.gateways)
	if err != nil {
		return nil, err
	}
	robots, err := s.robots.GatewayRobots(ctx, gatewayID)
	if err != nil {
		return nil, toStatus(err)
	}
	return &gatewayv1.ListRobotsResponse{Robots: robotsToProto(robots)}, nil
}

// ReportScan stores the calling gateway's robot list and returns every
// scanned robot with its robotId.
func (s *Server) ReportScan(ctx context.Context, req *gatewayv1.ReportScanRequest) (*gatewayv1.ReportScanResponse, error) {
	if err := validateScan(req.GetSerialNumbers()); err != nil {
		return nil, err
	}
	gatewayID, err := CallingGateway(ctx, s.gateways)
	if err != nil {
		return nil, err
	}

	// Every scanned robot gets a new ID; the service keeps the stored ID of
	// robots it already knows.
	scanned := make([]robotdomain.Robot, len(req.GetSerialNumbers()))
	for i, serial := range req.GetSerialNumbers() {
		scanned[i] = robotdomain.Robot{ID: uuid.New(), SerialNumber: serial, GatewayID: gatewayID}
	}
	robots, err := s.robots.ReportScan(ctx, scanned)
	if err != nil {
		return nil, toStatus(err)
	}
	return &gatewayv1.ReportScanResponse{Robots: robotsToProto(robots)}, nil
}

// CallingGateway reads the calling gateway's ID from the metadata and marks
// the gateway as seen. A missing or malformed ID is UNAUTHENTICATED, an
// unregistered gateway PERMISSION_DENIED. Other errors are returned as they are.
func CallingGateway(ctx context.Context, gateways GatewayService) (uuid.UUID, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get(GatewayIDKey)
	if len(values) != 1 {
		return uuid.UUID{}, status.Errorf(codes.Unauthenticated, "exactly one %s metadata value is required", GatewayIDKey)
	}
	id, err := uuid.Parse(values[0])
	if err != nil {
		return uuid.UUID{}, status.Errorf(codes.Unauthenticated, "%s must be a UUID", GatewayIDKey)
	}
	if _, err = gateways.MarkSeen(ctx, id); err != nil {
		if errors.Is(err, gatewaydomain.ErrNotFound) {
			return uuid.UUID{}, status.Error(codes.PermissionDenied, "gateway is not registered")
		}
		return uuid.UUID{}, err
	}
	return id, nil
}

// validateScan applies the contract's rules: no empty and no duplicate
// serial numbers.
func validateScan(serials []string) error {
	seen := make(map[string]bool, len(serials))
	for _, serial := range serials {
		if serial == "" {
			return status.Error(codes.InvalidArgument, "serial numbers must not be empty")
		}
		if seen[serial] {
			return status.Errorf(codes.InvalidArgument, "serial number %q appears twice", serial)
		}
		seen[serial] = true
	}
	return nil
}

// toStatus maps an error from the robot service to the contract's codes.
// Other errors are unexpected and returned as they are; the middleware logs
// them and answers INTERNAL.
func toStatus(err error) error {
	if errors.Is(err, robotdomain.ErrOtherGateway) {
		return status.Error(codes.PermissionDenied, "a scanned robot belongs to another gateway")
	}
	return err
}

func robotsToProto(robots []robotdomain.Robot) []*gatewayv1.Robot {
	out := make([]*gatewayv1.Robot, len(robots))
	for i, r := range robots {
		out[i] = &gatewayv1.Robot{RobotId: r.ID.String(), SerialNumber: r.SerialNumber}
	}
	return out
}
