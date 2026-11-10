// Package handler implements the gateway operations of the REST API
// (apispecs/rest/openapi.yaml): registering and reading gateways and
// listing a gateway's robots. The generated server in gen/fleet/v1 routes
// requests and validates them before calling these handlers.
package handler

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
}

// Handler handles the gateway operations of the REST API.
type Handler struct {
	gateways GatewayService
	robots   RobotService
}

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
		res := fleetv1.CreateGatewayConflict(Problem(http.StatusConflict, "a gateway with this serial number is already registered"))
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
		res := fleetv1.GetGatewayNotFound(Problem(http.StatusNotFound, "gateway not found"))
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
		res := fleetv1.ListGatewayRobotsNotFound(Problem(http.StatusNotFound, "gateway not found"))
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
		items[i] = RobotToREST(r, now)
	}
	return &fleetv1.RobotList{Items: items}, nil
}

// Problem returns a problem details body (RFC 9457) for the status code;
// an empty detail is left out.
func Problem(code int, detail string) fleetv1.Problem {
	p := fleetv1.Problem{
		Title:  http.StatusText(code),
		Status: code,
	}
	if detail != "" {
		p.Detail = fleetv1.NewOptString(detail)
	}
	return p
}

// RobotToREST maps a robot to its REST representation at now, with its
// newest measurement as latest (null if none arrived yet).
func RobotToREST(r robotdomain.Robot, now time.Time) fleetv1.Robot {
	out := fleetv1.Robot{
		RobotId:      r.ID,
		SerialNumber: r.SerialNumber,
		GatewayId:    r.GatewayID,
		FirstSeenAt:  r.FirstSeenAt,
		Connectivity: fleetv1.Connectivity(gatewaydomain.ConnectivityAt(r.LastSeenAt, now)),
		LastSeenAt:   optTime(r.LastSeenAt),
	}
	if r.Latest != nil {
		out.Latest = fleetv1.NewOptNilMeasurement(MeasurementToREST(*r.Latest))
	} else {
		out.Latest.SetToNull()
	}
	return out
}

// MeasurementToREST maps a measurement to its REST representation; values
// that were not reported are null.
func MeasurementToREST(m robotdomain.Measurement) fleetv1.Measurement {
	out := fleetv1.Measurement{
		Activity:         fleetv1.Activity(m.Activity),
		ActiveFaultCodes: m.ActiveFaultCodes,
		MeasuredAt:       m.MeasuredAt,
	}
	if out.ActiveFaultCodes == nil {
		out.ActiveFaultCodes = []string{}
	}
	if m.BatteryPct != nil {
		out.BatteryPct = fleetv1.NewOptNilFloat64(*m.BatteryPct)
	} else {
		out.BatteryPct.SetToNull()
	}
	if p := m.Position; p != nil {
		out.Position = fleetv1.NewOptNilPosition(fleetv1.Position{MapId: p.MapID, X: p.X, Y: p.Y, HeadingRad: p.HeadingRad})
	} else {
		out.Position.SetToNull()
	}
	return out
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

// optTime returns null for a nil time, so the field is always present.
func optTime(t *time.Time) fleetv1.OptNilDateTime {
	if t == nil {
		var out fleetv1.OptNilDateTime
		out.SetToNull()
		return out
	}
	return fleetv1.NewOptNilDateTime(*t)
}
