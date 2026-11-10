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
	Err          error
	Robots       []domain.Robot
	Measurements []domain.Measurement
	Faults       []domain.Fault
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

// CreateMeasurement stores m unless a measurement of the same robot with the
// same MeasuredAt is stored.
func (f *FakeRepository) CreateMeasurement(_ context.Context, m domain.Measurement) (domain.Measurement, error) {
	if f.Err != nil {
		return domain.Measurement{}, f.Err
	}
	for _, stored := range f.Measurements {
		if stored.RobotID == m.RobotID && stored.MeasuredAt.Equal(m.MeasuredAt) {
			return m, nil
		}
	}
	f.Measurements = append(f.Measurements, m)
	return m, nil
}

// ListMeasurements returns the robot's measurements from from (inclusive) to
// to (exclusive), in insertion order.
func (f *FakeRepository) ListMeasurements(_ context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Measurement, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	measurements := []domain.Measurement{}
	for _, m := range f.Measurements {
		if m.RobotID == robotID && !m.MeasuredAt.Before(from) && m.MeasuredAt.Before(to) {
			measurements = append(measurements, m)
		}
	}
	return measurements, nil
}

// CreateFault stores fault unless a fault of the same robot with the same
// Code and OccurredAt is stored.
func (f *FakeRepository) CreateFault(_ context.Context, fault domain.Fault) (domain.Fault, error) {
	if f.Err != nil {
		return domain.Fault{}, f.Err
	}
	for _, stored := range f.Faults {
		if stored.RobotID == fault.RobotID && stored.Code == fault.Code && stored.OccurredAt.Equal(fault.OccurredAt) {
			return fault, nil
		}
	}
	f.Faults = append(f.Faults, fault)
	return fault, nil
}

// ListFaults returns the robot's faults from from (inclusive) to to
// (exclusive), in insertion order.
func (f *FakeRepository) ListFaults(_ context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Fault, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	faults := []domain.Fault{}
	for _, fault := range f.Faults {
		if fault.RobotID == robotID && !fault.OccurredAt.Before(from) && fault.OccurredAt.Before(to) {
			faults = append(faults, fault)
		}
	}
	return faults, nil
}
