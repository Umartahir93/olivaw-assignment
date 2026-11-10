package robot

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// FakeRepository is an in-memory Repository for tests. Set Err to make
// every call fail with it.
type FakeRepository struct {
	Err    error
	Robots []domain.Robot
}

var _ Repository = (*FakeRepository)(nil)

// CreateRobots stores robots.
func (f *FakeRepository) CreateRobots(_ context.Context, robots []domain.Robot) ([]domain.Robot, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	f.Robots = append(f.Robots, robots...)
	return robots, nil
}

// GetRobot returns the robot with this ID, or domain.ErrNotFound.
func (f *FakeRepository) GetRobot(_ context.Context, id uuid.UUID) (domain.Robot, error) {
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

// GetRobotsBySerial returns the stored robots with these serial numbers.
func (f *FakeRepository) GetRobotsBySerial(_ context.Context, serialNumbers []string) ([]domain.Robot, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	var found []domain.Robot
	for _, r := range f.Robots {
		if slices.Contains(serialNumbers, r.SerialNumber) {
			found = append(found, r)
		}
	}
	return found, nil
}

// ListRobots returns the gateway's robots in insertion order.
func (f *FakeRepository) ListRobots(_ context.Context, gatewayID uuid.UUID) ([]domain.Robot, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	robots := []domain.Robot{}
	for _, r := range f.Robots {
		if r.GatewayID == gatewayID {
			robots = append(robots, r)
		}
	}
	return robots, nil
}

// UpdateRobotLastSeen sets the robot's LastSeenAt to at, or returns domain.ErrNotFound.
func (f *FakeRepository) UpdateRobotLastSeen(_ context.Context, id uuid.UUID, at time.Time) (domain.Robot, error) {
	if f.Err != nil {
		return domain.Robot{}, f.Err
	}
	for i := range f.Robots {
		if f.Robots[i].ID == id {
			f.Robots[i].LastSeenAt = &at
			return f.Robots[i], nil
		}
	}
	return domain.Robot{}, domain.ErrNotFound
}
