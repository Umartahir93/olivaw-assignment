package handler

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// FakeRobotService is an in-memory RobotService for tests. It knows the
// robots, measurements and faults it holds; set Err to make every call fail
// with it.
type FakeRobotService struct {
	Err          error
	Robots       []domain.Robot
	Measurements []domain.Measurement
	Faults       []domain.Fault

	// From and To hold the time range of the last list call.
	From, To time.Time
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

// ListMeasurements records the range and returns the robot's measurements,
// or domain.ErrNotFound for an unknown robot.
func (f *FakeRobotService) ListMeasurements(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Measurement, error) {
	f.From, f.To = from, to
	if _, err := f.GetRobot(ctx, robotID); err != nil {
		return nil, err
	}
	measurements := []domain.Measurement{}
	for _, m := range f.Measurements {
		if m.RobotID == robotID {
			measurements = append(measurements, m)
		}
	}
	return measurements, nil
}

// ListFaults records the range and returns the robot's faults, or
// domain.ErrNotFound for an unknown robot.
func (f *FakeRobotService) ListFaults(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Fault, error) {
	f.From, f.To = from, to
	if _, err := f.GetRobot(ctx, robotID); err != nil {
		return nil, err
	}
	faults := []domain.Fault{}
	for _, fault := range f.Faults {
		if fault.RobotID == robotID {
			faults = append(faults, fault)
		}
	}
	return faults, nil
}
