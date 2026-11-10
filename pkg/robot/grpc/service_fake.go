package grpc

import (
	"context"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// FakeRobotService is a RobotService for tests. It fails with Err, or
// records what it is asked to store.
type FakeRobotService struct {
	Err          error
	Measurements []domain.Measurement
	Faults       []domain.Fault
}

var _ RobotService = (*FakeRobotService)(nil)

// ReportMeasurement records m.
func (f *FakeRobotService) ReportMeasurement(_ context.Context, _ uuid.UUID, m domain.Measurement) error {
	if f.Err != nil {
		return f.Err
	}
	f.Measurements = append(f.Measurements, m)
	return nil
}

// ReportFault records fault.
func (f *FakeRobotService) ReportFault(_ context.Context, _ uuid.UUID, fault domain.Fault) error {
	if f.Err != nil {
		return f.Err
	}
	f.Faults = append(f.Faults, fault)
	return nil
}
