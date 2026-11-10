// Package adapter holds the gRPC handlers for gateways.
package adapter

import (
	"context"

	"github.com/google/uuid"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
)

// GatewayServer is a stub GatewayService.
type GatewayServer struct {
	gatewayv1.UnimplementedGatewayServiceServer
}

// ListRobots returns no robots: the stub keeps no state.
func (s *GatewayServer) ListRobots(context.Context, *gatewayv1.ListRobotsRequest) (*gatewayv1.ListRobotsResponse, error) {
	return &gatewayv1.ListRobotsResponse{}, nil
}

// ReportScan returns every scanned robot with a new robotId.
func (s *GatewayServer) ReportScan(_ context.Context, req *gatewayv1.ReportScanRequest) (*gatewayv1.ReportScanResponse, error) {
	robots := make([]*gatewayv1.Robot, 0, len(req.GetSerialNumbers()))
	for _, serial := range req.GetSerialNumbers() {
		robots = append(robots, &gatewayv1.Robot{RobotId: uuid.NewString(), SerialNumber: serial})
	}
	return &gatewayv1.ReportScanResponse{Robots: robots}, nil
}

// RobotServer is a stub RobotService: it answers every request with an
// empty response and does nothing else.
type RobotServer struct {
	gatewayv1.UnimplementedRobotServiceServer
}

func (s *RobotServer) ReportMeasurement(context.Context, *gatewayv1.ReportMeasurementRequest) (*gatewayv1.ReportMeasurementResponse, error) {
	return &gatewayv1.ReportMeasurementResponse{}, nil
}

func (s *RobotServer) ReportFault(context.Context, *gatewayv1.ReportFaultRequest) (*gatewayv1.ReportFaultResponse, error) {
	return &gatewayv1.ReportFaultResponse{}, nil
}
