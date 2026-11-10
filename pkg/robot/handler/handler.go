// Package handler implements the robot operations of the REST API
// (apispecs/rest/openapi.yaml): reading a robot and its measurements and
// faults. Measurements and faults are not stored yet, so their lists are
// always empty.
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
	_, err := h.robots.GetRobot(ctx, params.RobotId)
	if errors.Is(err, domain.ErrNotFound) {
		res := fleetv1.ListRobotMeasurementsNotFound(notFound())
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
	if errors.Is(err, domain.ErrNotFound) {
		res := fleetv1.ListRobotFaultsNotFound(notFound())
		return &res, nil
	}
	if err != nil {
		return nil, err
	}
	return &fleetv1.FaultList{Items: []fleetv1.Fault{}}, nil
}

func notFound() fleetv1.Problem {
	return gatewayhandler.Problem(http.StatusNotFound, "robot not found")
}
