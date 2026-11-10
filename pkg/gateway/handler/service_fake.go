package handler

import (
	"context"

	"github.com/google/uuid"

	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	robotdomain "github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// FakeGatewayService is an in-memory GatewayService for tests. It knows the
// gateways in Gateways; set Err to make every call fail with it.
type FakeGatewayService struct {
	Err      error
	Gateways []gatewaydomain.Gateway
}

var _ GatewayService = (*FakeGatewayService)(nil)

// RegisterGateway stores g.
func (f *FakeGatewayService) RegisterGateway(_ context.Context, g gatewaydomain.Gateway) (gatewaydomain.Gateway, error) {
	if f.Err != nil {
		return gatewaydomain.Gateway{}, f.Err
	}
	f.Gateways = append(f.Gateways, g)
	return g, nil
}

// GetGateway returns the gateway with this ID, or domain.ErrNotFound.
func (f *FakeGatewayService) GetGateway(_ context.Context, id uuid.UUID) (gatewaydomain.Gateway, error) {
	if f.Err != nil {
		return gatewaydomain.Gateway{}, f.Err
	}
	for _, g := range f.Gateways {
		if g.ID == id {
			return g, nil
		}
	}
	return gatewaydomain.Gateway{}, gatewaydomain.ErrNotFound
}

// ListGateways returns Gateways.
func (f *FakeGatewayService) ListGateways(context.Context) ([]gatewaydomain.Gateway, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Gateways, nil
}

// FakeRobotService is a RobotService for tests. It answers with Robots, or
// fails with Err.
type FakeRobotService struct {
	Err    error
	Robots []robotdomain.Robot
}

var _ RobotService = (*FakeRobotService)(nil)

// GatewayRobots returns Robots.
func (f *FakeRobotService) GatewayRobots(context.Context, uuid.UUID) ([]robotdomain.Robot, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Robots, nil
}
