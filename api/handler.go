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

// CreateGateway handles POST /v1/gateways.
func (h *Handler) CreateGateway(_ context.Context, req *fleetv1.GatewayCreate) (fleetv1.CreateGatewayRes, error) {
	gateway := stubGateway()
	gateway.SerialNumber = req.SerialNumber
	gateway.Location = req.Location
	gateway.Name = req.Name
	return &fleetv1.GatewayHeaders{
		Location: fleetv1.NewOptURI(url.URL{Path: "/v1/gateways/" + gateway.GatewayId.String()}),
		Response: gateway,
	}, nil
}

// ListGateways handles GET /v1/gateways.
func (h *Handler) ListGateways(context.Context, fleetv1.ListGatewaysParams) (fleetv1.ListGatewaysRes, error) {
	return &fleetv1.GatewayPage{Items: []fleetv1.Gateway{}}, nil
}

// GetGateway handles GET /v1/gateways/{gatewayId}.
func (h *Handler) GetGateway(_ context.Context, params fleetv1.GetGatewayParams) (fleetv1.GetGatewayRes, error) {
	gateway := stubGateway()
	gateway.GatewayId = params.GatewayId
	return &gateway, nil
}

// ListGatewayRobots handles GET /v1/gateways/{gatewayId}/robots.
func (h *Handler) ListGatewayRobots(context.Context, fleetv1.ListGatewayRobotsParams) (fleetv1.ListGatewayRobotsRes, error) {
	return &fleetv1.RobotPage{Items: []fleetv1.Robot{}}, nil
}

// GetRobot handles GET /v1/robots/{robotId}.
func (h *Handler) GetRobot(_ context.Context, params fleetv1.GetRobotParams) (fleetv1.GetRobotRes, error) {
	return &fleetv1.Robot{
		RobotId:      params.RobotId,
		SerialNumber: "stub",
		GatewayId:    uuid.New(),
		FirstSeenAt:  time.Now().UTC(),
		Connectivity: fleetv1.ConnectivityUnknown,
	}, nil
}

// ListRobotMeasurements handles GET /v1/robots/{robotId}/measurements.
func (h *Handler) ListRobotMeasurements(context.Context, fleetv1.ListRobotMeasurementsParams) (fleetv1.ListRobotMeasurementsRes, error) {
	return &fleetv1.MeasurementPage{Items: []fleetv1.Measurement{}}, nil
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

// stubGateway is a newly registered gateway that has not called yet.
func stubGateway() fleetv1.Gateway {
	return fleetv1.Gateway{
		GatewayId:    uuid.New(),
		SerialNumber: "stub",
		Location:     "stub",
		RegisteredAt: time.Now().UTC(),
		Connectivity: fleetv1.ConnectivityUnknown,
	}
}
