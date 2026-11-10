package grpc

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

// MarkSeen returns the gateway with this ID, or domain.ErrNotFound.
func (f *FakeGatewayService) MarkSeen(_ context.Context, id uuid.UUID) (gatewaydomain.Gateway, error) {
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

// FakeRobotService is a RobotService for tests. It answers with Robots, or
// fails with Err, and records the robots of the last ReportScan in Scanned.
type FakeRobotService struct {
	Err     error
	Robots  []robotdomain.Robot
	Scanned []robotdomain.Robot
}

var _ RobotService = (*FakeRobotService)(nil)

// GatewayRobots returns Robots.
func (f *FakeRobotService) GatewayRobots(context.Context, uuid.UUID) ([]robotdomain.Robot, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Robots, nil
}

// ReportScan records scanned and returns Robots.
func (f *FakeRobotService) ReportScan(_ context.Context, scanned []robotdomain.Robot) ([]robotdomain.Robot, error) {
	f.Scanned = scanned
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Robots, nil
}
