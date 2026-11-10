// Package api implements the fleet REST API (apispecs/rest/openapi.yaml). The
// generated server in gen/fleet/v1 routes requests and validates parameters
// and bodies before calling these handlers.
//
// This is a stub: every handler returns a fixed successful response.
package api

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
)

// Handler implements fleetv1.Handler.
type Handler struct{}

var _ fleetv1.Handler = (*Handler)(nil)

// NewHandler returns a Handler.
func NewHandler() *Handler {
	return &Handler{}
}

// ListRobots handles GET /v1/robots.
func (h *Handler) ListRobots(context.Context, fleetv1.ListRobotsParams) (fleetv1.ListRobotsRes, error) {
	return &fleetv1.RobotPage{Items: []fleetv1.Robot{}}, nil
}

// CreateRobot handles POST /v1/robots.
func (h *Handler) CreateRobot(context.Context, *fleetv1.RobotCreate) (fleetv1.CreateRobotRes, error) {
	robot := stubRobot()
	return &fleetv1.RobotHeaders{
		Location: fleetv1.NewOptURI(url.URL{Path: "/v1/robots/" + robot.RobotId.String()}),
		Response: robot,
	}, nil
}

// GetRobot handles GET /v1/robots/{robotId}.
func (h *Handler) GetRobot(context.Context, fleetv1.GetRobotParams) (fleetv1.GetRobotRes, error) {
	robot := stubRobot()
	return &robot, nil
}

// DecommissionRobot handles DELETE /v1/robots/{robotId}.
func (h *Handler) DecommissionRobot(context.Context, fleetv1.DecommissionRobotParams) (fleetv1.DecommissionRobotRes, error) {
	return &fleetv1.DecommissionRobotNoContent{}, nil
}

// ListRobotTelemetry handles GET /v1/robots/{robotId}/telemetry.
func (h *Handler) ListRobotTelemetry(context.Context, fleetv1.ListRobotTelemetryParams) (fleetv1.ListRobotTelemetryRes, error) {
	return &fleetv1.TelemetryPage{Items: []fleetv1.TelemetryReading{}}, nil
}

// ListRobotFaults handles GET /v1/robots/{robotId}/faults.
func (h *Handler) ListRobotFaults(context.Context, fleetv1.ListRobotFaultsParams) (fleetv1.ListRobotFaultsRes, error) {
	return &fleetv1.FaultPage{Items: []fleetv1.Fault{}}, nil
}

// NewError turns an error returned by a handler into a 500 problem.
func (h *Handler) NewError(_ context.Context, _ error) *fleetv1.ErrorStatusCode {
	return &fleetv1.ErrorStatusCode{
		StatusCode: http.StatusInternalServerError,
		Response: fleetv1.Problem{
			Title:  http.StatusText(http.StatusInternalServerError),
			Status: http.StatusInternalServerError,
		},
	}
}

// stubRobot is a newly onboarded robot that has not sent any data yet.
func stubRobot() fleetv1.Robot {
	return fleetv1.Robot{
		RobotId:      uuid.New(),
		SerialNumber: "stub",
		Location:     "stub",
		Status:       fleetv1.RobotStatusActive,
		RegisteredAt: time.Now().UTC(),
		Connectivity: fleetv1.ConnectivityNeverSeen,
	}
}
