package telemetry

import (
	"context"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/olivaw/fleet/v1"
)

// Server is a stub TelemetryService: it answers every request
// with an empty response and does nothing else.
type Server struct {
	fleetv1.UnimplementedTelemetryServiceServer
}

func (s *Server) ReportTelemetry(context.Context, *fleetv1.ReportTelemetryRequest) (*fleetv1.ReportTelemetryResponse, error) {
	return &fleetv1.ReportTelemetryResponse{}, nil
}

func (s *Server) ReportFault(context.Context, *fleetv1.ReportFaultRequest) (*fleetv1.ReportFaultResponse, error) {
	return &fleetv1.ReportFaultResponse{}, nil
}
