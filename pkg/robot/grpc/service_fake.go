package grpc

import (
	"context"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// FakeRobotService is a RobotService for tests. It fails with Err, or
// records the robot IDs passed to MarkSeen in Seen.
type FakeRobotService struct {
	Err  error
	Seen []uuid.UUID
}

var _ RobotService = (*FakeRobotService)(nil)

// MarkSeen records robotID and returns the robot of gatewayID.
func (f *FakeRobotService) MarkSeen(_ context.Context, gatewayID, robotID uuid.UUID) (domain.Robot, error) {
	if f.Err != nil {
		return domain.Robot{}, f.Err
	}
	f.Seen = append(f.Seen, robotID)
	return domain.Robot{ID: robotID, GatewayID: gatewayID}, nil
}
