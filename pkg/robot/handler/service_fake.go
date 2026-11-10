package handler

import (
	"context"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// FakeRobotService is an in-memory RobotService for tests. It knows the
// robots in Robots; set Err to make every call fail with it.
type FakeRobotService struct {
	Err    error
	Robots []domain.Robot
}

var _ RobotService = (*FakeRobotService)(nil)

// GetRobot returns the robot with this ID, or domain.ErrNotFound.
func (f *FakeRobotService) GetRobot(_ context.Context, id uuid.UUID) (domain.Robot, error) {
	if f.Err != nil {
		return domain.Robot{}, f.Err
	}
	for _, r := range f.Robots {
		if r.ID == id {
			return r, nil
		}
	}
	return domain.Robot{}, domain.ErrNotFound
}
