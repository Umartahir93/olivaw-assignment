package telemetry

import (
	"context"
	"testing"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/olivaw/fleet/v1"
)

func TestReportTelemetryReturnsResponse(t *testing.T) {
	resp, err := (&Server{}).ReportTelemetry(context.Background(), &fleetv1.ReportTelemetryRequest{})
	if err != nil {
		t.Fatalf("ReportTelemetry: %v", err)
	}
	if resp == nil {
		t.Fatal("ReportTelemetry returned no response")
	}
}

func TestReportFaultReturnsResponse(t *testing.T) {
	resp, err := (&Server{}).ReportFault(context.Background(), &fleetv1.ReportFaultRequest{})
	if err != nil {
		t.Fatalf("ReportFault: %v", err)
	}
	if resp == nil {
		t.Fatal("ReportFault returned no response")
	}
}
