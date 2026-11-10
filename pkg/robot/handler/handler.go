// Package handler implements the robot operations of the REST API
// (apispecs/rest/openapi.yaml): reading a robot and its measurements and
// faults.
package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	gatewayhandler "github.com/umartahir93/olivaw-assignment/pkg/gateway/handler"
	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// RobotService is what the handlers need from the robot service.
type RobotService interface {
	GetRobot(ctx context.Context, id uuid.UUID) (domain.Robot, error)
	ListMeasurements(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Measurement, error)
	ListFaults(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Fault, error)
}

// Handler handles the robot operations of the REST API.
type Handler struct {
	robots RobotService
}

// NewHandler returns a Handler backed by the robot service.
func NewHandler(robots RobotService) *Handler {
	return &Handler{robots: robots}
}

// GetRobot handles GET /v1/robots/{robotId}.
func (h *Handler) GetRobot(ctx context.Context, params fleetv1.GetRobotParams) (fleetv1.GetRobotRes, error) {
	r, err := h.robots.GetRobot(ctx, params.RobotId)
	if errors.Is(err, domain.ErrNotFound) {
		res := fleetv1.GetRobotNotFound(notFound())
		return &res, nil
	}
	if err != nil {
		return nil, err
	}
	res := gatewayhandler.RobotToREST(r, time.Now())
	return &res, nil
}

// ListRobotMeasurements handles GET /v1/robots/{robotId}/measurements.
func (h *Handler) ListRobotMeasurements(ctx context.Context, params fleetv1.ListRobotMeasurementsParams) (fleetv1.ListRobotMeasurementsRes, error) {
	from, to, problem := timeRange(params.From, params.To, time.Now())
	if problem != "" {
		res := fleetv1.ListRobotMeasurementsBadRequest(gatewayhandler.Problem(http.StatusBadRequest, problem))
		return &res, nil
	}
	measurements, err := h.robots.ListMeasurements(ctx, params.RobotId, from, to)
	if errors.Is(err, domain.ErrNotFound) {
		res := fleetv1.ListRobotMeasurementsNotFound(notFound())
		return &res, nil
	}
	if err != nil {
		return nil, err
	}

	items := make([]fleetv1.Measurement, len(measurements))
	for i, m := range measurements {
		items[i] = gatewayhandler.MeasurementToREST(m)
	}
	return &fleetv1.MeasurementList{Items: items}, nil
}

// ListRobotFaults handles GET /v1/robots/{robotId}/faults.
func (h *Handler) ListRobotFaults(ctx context.Context, params fleetv1.ListRobotFaultsParams) (fleetv1.ListRobotFaultsRes, error) {
	from, to, problem := timeRange(params.From, params.To, time.Now())
	if problem != "" {
		res := fleetv1.ListRobotFaultsBadRequest(gatewayhandler.Problem(http.StatusBadRequest, problem))
		return &res, nil
	}
	faults, err := h.robots.ListFaults(ctx, params.RobotId, from, to)
	if errors.Is(err, domain.ErrNotFound) {
		res := fleetv1.ListRobotFaultsNotFound(notFound())
		return &res, nil
	}
	if err != nil {
		return nil, err
	}

	items := make([]fleetv1.Fault, len(faults))
	for i, f := range faults {
		items[i] = faultToREST(f)
	}
	return &fleetv1.FaultList{Items: items}, nil
}

// timeRange applies the contract's rules for from and to: to defaults to
// now, from to domain.MaxTimeRange before to. It returns a problem detail if
// the range ends before it starts or is longer than domain.MaxTimeRange.
func timeRange(fromParam, toParam fleetv1.OptDateTime, now time.Time) (from, to time.Time, problem string) {
	to = now
	if t, ok := toParam.Get(); ok {
		to = t
	}
	from = to.Add(-domain.MaxTimeRange)
	if f, ok := fromParam.Get(); ok {
		from = f
	}
	switch {
	case to.Before(from):
		return time.Time{}, time.Time{}, "to must not be before from"
	case to.Sub(from) > domain.MaxTimeRange:
		return time.Time{}, time.Time{}, "the time range must not be longer than 24 hours"
	}
	return from, to, ""
}

func faultToREST(f domain.Fault) fleetv1.Fault {
	return fleetv1.Fault{
		Code:       f.Code,
		Severity:   fleetv1.FaultSeverity(f.Severity),
		Message:    f.Message,
		OccurredAt: f.OccurredAt,
	}
}

func notFound() fleetv1.Problem {
	return gatewayhandler.Problem(http.StatusNotFound, "robot not found")
}
