// Package api implements the fleet REST API (apispecs/rest/openapi.yaml). The
// generated server in gen/fleet/v1 routes requests and validates parameters
// and bodies before calling these handlers.
//
// Measurements and faults are not stored yet, so their lists are always
// empty.
package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	robotdomain "github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// GatewayService is what the handlers need from the gateway service.
type GatewayService interface {
	RegisterGateway(ctx context.Context, g gatewaydomain.Gateway) (gatewaydomain.Gateway, error)
	GetGateway(ctx context.Context, id uuid.UUID) (gatewaydomain.Gateway, error)
	ListGateways(ctx context.Context) ([]gatewaydomain.Gateway, error)
}

// RobotService is what the handlers need from the robot service.
type RobotService interface {
	GatewayRobots(ctx context.Context, gatewayID uuid.UUID) ([]robotdomain.Robot, error)
	GetRobot(ctx context.Context, id uuid.UUID) (robotdomain.Robot, error)
}

// Handler implements fleetv1.Handler.
type Handler struct {
	gateways GatewayService
	robots   RobotService
}

var _ fleetv1.Handler = (*Handler)(nil)

// NewHandler returns a Handler backed by the gateway and robot services.
func NewHandler(gateways GatewayService, robots RobotService) *Handler {
	return &Handler{gateways: gateways, robots: robots}
}

// CreateGateway handles POST /v1/gateways.
func (h *Handler) CreateGateway(ctx context.Context, req *fleetv1.GatewayCreate) (fleetv1.CreateGatewayRes, error) {
	g := gatewaydomain.Gateway{
		ID:           uuid.New(),
		SerialNumber: req.SerialNumber,
		Location:     req.Location,
	}
	if name, ok := req.Name.Get(); ok {
		g.Name = &name
	}
	created, err := h.gateways.RegisterGateway(ctx, g)
	if errors.Is(err, gatewaydomain.ErrAlreadyExists) {
		res := fleetv1.CreateGatewayConflict(problem(http.StatusConflict, "a gateway with this serial number is already registered"))
		return &res, nil
	}
	if err != nil {
		return nil, err
	}
	return &fleetv1.GatewayHeaders{
		Location: fleetv1.NewOptURI(url.URL{Path: "/v1/gateways/" + created.ID.String()}),
		Response: gatewayToREST(created, time.Now()),
	}, nil
}

// ListGateways handles GET /v1/gateways.
func (h *Handler) ListGateways(ctx context.Context) (*fleetv1.GatewayList, error) {
	gateways, err := h.gateways.ListGateways(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	items := make([]fleetv1.Gateway, len(gateways))
	for i, g := range gateways {
		items[i] = gatewayToREST(g, now)
	}
	return &fleetv1.GatewayList{Items: items}, nil
}

// GetGateway handles GET /v1/gateways/{gatewayId}.
func (h *Handler) GetGateway(ctx context.Context, params fleetv1.GetGatewayParams) (fleetv1.GetGatewayRes, error) {
	g, err := h.gateways.GetGateway(ctx, params.GatewayId)
	if errors.Is(err, gatewaydomain.ErrNotFound) {
		res := fleetv1.GetGatewayNotFound(problem(http.StatusNotFound, "gateway not found"))
		return &res, nil
	}
	if err != nil {
		return nil, err
	}
	res := gatewayToREST(g, time.Now())
	return &res, nil
}

// ListGatewayRobots handles GET /v1/gateways/{gatewayId}/robots.
func (h *Handler) ListGatewayRobots(ctx context.Context, params fleetv1.ListGatewayRobotsParams) (fleetv1.ListGatewayRobotsRes, error) {
	_, err := h.gateways.GetGateway(ctx, params.GatewayId)
	if errors.Is(err, gatewaydomain.ErrNotFound) {
		res := fleetv1.ListGatewayRobotsNotFound(problem(http.StatusNotFound, "gateway not found"))
		return &res, nil
	}
	if err != nil {
		return nil, err
	}
	robots, err := h.robots.GatewayRobots(ctx, params.GatewayId)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	items := make([]fleetv1.Robot, len(robots))
	for i, r := range robots {
		items[i] = robotToREST(r, now)
	}
	return &fleetv1.RobotList{Items: items}, nil
}

// GetRobot handles GET /v1/robots/{robotId}.
func (h *Handler) GetRobot(ctx context.Context, params fleetv1.GetRobotParams) (fleetv1.GetRobotRes, error) {
	r, err := h.robots.GetRobot(ctx, params.RobotId)
	if errors.Is(err, robotdomain.ErrNotFound) {
		res := fleetv1.GetRobotNotFound(problem(http.StatusNotFound, "robot not found"))
		return &res, nil
	}
	if err != nil {
		return nil, err
	}
	res := robotToREST(r, time.Now())
	return &res, nil
}

// ListRobotMeasurements handles GET /v1/robots/{robotId}/measurements.
func (h *Handler) ListRobotMeasurements(ctx context.Context, params fleetv1.ListRobotMeasurementsParams) (fleetv1.ListRobotMeasurementsRes, error) {
	_, err := h.robots.GetRobot(ctx, params.RobotId)
	if errors.Is(err, robotdomain.ErrNotFound) {
		res := fleetv1.ListRobotMeasurementsNotFound(problem(http.StatusNotFound, "robot not found"))
		return &res, nil
	}
	if err != nil {
		return nil, err
	}
	return &fleetv1.MeasurementList{Items: []fleetv1.Measurement{}}, nil
}

// ListRobotFaults handles GET /v1/robots/{robotId}/faults.
func (h *Handler) ListRobotFaults(ctx context.Context, params fleetv1.ListRobotFaultsParams) (fleetv1.ListRobotFaultsRes, error) {
	_, err := h.robots.GetRobot(ctx, params.RobotId)
	if errors.Is(err, robotdomain.ErrNotFound) {
		res := fleetv1.ListRobotFaultsNotFound(problem(http.StatusNotFound, "robot not found"))
		return &res, nil
	}
	if err != nil {
		return nil, err
	}
	return &fleetv1.FaultList{Items: []fleetv1.Fault{}}, nil
}

// NewError turns an error returned by a handler into a 500 problem.
func (h *Handler) NewError(_ context.Context, _ error) *fleetv1.ErrorStatusCode {
	return &fleetv1.ErrorStatusCode{
		StatusCode: http.StatusInternalServerError,
		Response:   problem(http.StatusInternalServerError, ""),
	}
}

func problem(code int, detail string) fleetv1.Problem {
	p := fleetv1.Problem{
		Title:  http.StatusText(code),
		Status: code,
	}
	if detail != "" {
		p.Detail = fleetv1.NewOptString(detail)
	}
	return p
}

func gatewayToREST(g gatewaydomain.Gateway, now time.Time) fleetv1.Gateway {
	out := fleetv1.Gateway{
		GatewayId:    g.ID,
		SerialNumber: g.SerialNumber,
		Location:     g.Location,
		RegisteredAt: g.RegisteredAt,
		Connectivity: fleetv1.Connectivity(g.Connectivity(now)),
		LastSeenAt:   optTime(g.LastSeenAt),
	}
	if g.Name != nil {
		out.Name = fleetv1.NewOptString(*g.Name)
	}
	return out
}

func robotToREST(r robotdomain.Robot, now time.Time) fleetv1.Robot {
	out := fleetv1.Robot{
		RobotId:      r.ID,
		SerialNumber: r.SerialNumber,
		GatewayId:    r.GatewayID,
		FirstSeenAt:  r.FirstSeenAt,
		Connectivity: fleetv1.Connectivity(gatewaydomain.ConnectivityAt(r.LastSeenAt, now)),
		LastSeenAt:   optTime(r.LastSeenAt),
	}
	// No measurements are stored yet.
	out.Latest.SetToNull()
	return out
}

// optTime returns null for a nil time, so the field is always present.
func optTime(t *time.Time) fleetv1.OptNilDateTime {
	if t == nil {
		var out fleetv1.OptNilDateTime
		out.SetToNull()
		return out
	}
	return fleetv1.NewOptNilDateTime(*t)
}
