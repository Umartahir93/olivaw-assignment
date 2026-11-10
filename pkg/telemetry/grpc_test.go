package telemetry

import (
	"context"
	"testing"

	telemetryv1 "github.com/umartahir93/olivaw-assignment/gen/telemetry/v1"
)

func TestReportTelemetryReturnsResponse(t *testing.T) {
	resp, err := (&Server{}).ReportTelemetry(context.Background(), &telemetryv1.ReportTelemetryRequest{})
	if err != nil {
		t.Fatalf("ReportTelemetry: %v", err)
	}
	if resp == nil {
		t.Fatal("ReportTelemetry returned no response")
	}
}

func TestReportFaultReturnsResponse(t *testing.T) {
	resp, err := (&Server{}).ReportFault(context.Background(), &telemetryv1.ReportFaultRequest{})
	if err != nil {
		t.Fatalf("ReportFault: %v", err)
	}
	if resp == nil {
		t.Fatal("ReportFault returned no response")
	}
}
