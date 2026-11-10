package telemetry

import (
	"context"

	telemetryv1 "github.com/umartahir93/olivaw-assignment/gen/telemetry/v1"
)

// Server is a stub TelemetryService: it answers every request
// with an empty response and does nothing else.
type Server struct {
	telemetryv1.UnimplementedTelemetryServiceServer
}

func (s *Server) ReportTelemetry(context.Context, *telemetryv1.ReportTelemetryRequest) (*telemetryv1.ReportTelemetryResponse, error) {
	return &telemetryv1.ReportTelemetryResponse{}, nil
}

func (s *Server) ReportFault(context.Context, *telemetryv1.ReportFaultRequest) (*telemetryv1.ReportFaultResponse, error) {
	return &telemetryv1.ReportFaultResponse{}, nil
}
