// Package robot is the service layer for robots: it discovers them from
// their gateway's scans, stores their measurements and faults, and answers
// questions about them.
//
// Input is validated by the handlers; this package applies business rules.
package robot

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// Repository stores robots, their measurements and their faults. Robot
// lists are ordered oldest first by FirstSeenAt; measurement and fault lists
// by their time, and cover from (inclusive) to to (exclusive). A robot's
// LastSeenAt is the later of when a scan last reported it and when its
// newest measurement arrived. Storing a measurement or fault that is already
// stored changes nothing. It reports an unknown robot as domain.ErrNotFound.
// Any other error means the storage itself failed.
type Repository interface {
	CreateRobots(ctx context.Context, robots []domain.Robot) ([]domain.Robot, error)
	GetRobot(ctx context.Context, id uuid.UUID) (domain.Robot, error)
	GetRobotsBySerial(ctx context.Context, serialNumbers []string) ([]domain.Robot, error)
	ListRobots(ctx context.Context, gatewayID uuid.UUID) ([]domain.Robot, error)
	UpdateRobotLastSeen(ctx context.Context, id uuid.UUID, at time.Time) (domain.Robot, error)
	CreateMeasurement(ctx context.Context, m domain.Measurement) (domain.Measurement, error)
	ListMeasurements(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Measurement, error)
	CreateFault(ctx context.Context, f domain.Fault) (domain.Fault, error)
	ListFaults(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Fault, error)
}

// Service handles robots.
type Service struct {
	repo Repository
}

// NewService returns a Service that stores robots in repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ReportScan takes the robots a gateway's scan found, each with an ID, its serial number and the gateway's ID, and returns them as the system
// knows them, in the same order. Robots seen for the first time are stored with the given ID; known robots keep their stored ID; robots
// missing from the scan are left as they are. Every scanned robot counts as
// seen now; after that, its measurements keep it seen.
//
// It returns domain.ErrOtherGateway if a scanned robot belongs to another
// gateway, and stores nothing in that case.
func (s *Service) ReportScan(ctx context.Context, scanned []domain.Robot) ([]domain.Robot, error) {
	serials := make([]string, len(scanned))
	for i, r := range scanned {
		serials[i] = r.SerialNumber
	}
	known, err := s.repo.GetRobotsBySerial(ctx, serials)
	if err != nil {
		return nil, err
	}
	bySerial := make(map[string]domain.Robot, len(known))
	for _, r := range known {
		bySerial[r.SerialNumber] = r
	}

	now := time.Now().UTC()
	var newRobots, knownRobots []domain.Robot
	for _, r := range scanned {
		k, ok := bySerial[r.SerialNumber]
		switch {
		case ok && k.GatewayID != r.GatewayID:
			return nil, domain.ErrOtherGateway
		case ok:
			knownRobots = append(knownRobots, k)
		default:
			newRobots = append(newRobots, domain.Robot{ID: r.ID, SerialNumber: r.SerialNumber, GatewayID: r.GatewayID, FirstSeenAt: now, LastSeenAt: &now})
		}
	}
	for _, k := range knownRobots {
		seen, err := s.repo.UpdateRobotLastSeen(ctx, k.ID, now)
		if err != nil {
			return nil, err
		}
		bySerial[seen.SerialNumber] = seen
	}
	if len(newRobots) > 0 {
		var created []domain.Robot
		if created, err = s.repo.CreateRobots(ctx, newRobots); err != nil {
			return nil, err
		}
		for _, r := range created {
			bySerial[r.SerialNumber] = r
		}
	}

	robots := make([]domain.Robot, len(scanned))
	for i, r := range scanned {
		robots[i] = bySerial[r.SerialNumber]
	}
	return robots, nil
}

// GatewayRobots returns all robots the system knows for a gateway, oldest
// first: for the fleet manager, and for a gateway recovering its robots' IDs
// after a restart. Whether the gateway exists is checked by the caller. For
// the sake of simplicity there is no pagination.
func (s *Service) GatewayRobots(ctx context.Context, gatewayID uuid.UUID) ([]domain.Robot, error) {
	return s.repo.ListRobots(ctx, gatewayID)
}

// GetRobot returns one robot, or domain.ErrNotFound.
func (s *Service) GetRobot(ctx context.Context, id uuid.UUID) (domain.Robot, error) {
	return s.repo.GetRobot(ctx, id)
}

// ReportMeasurement stores a measurement the gateway gatewayID sent for one
// of its robots, with the time it arrived. A measurement already stored is
// not stored again. It returns domain.ErrNotFound for an unknown robot and
// domain.ErrOtherGateway if the robot belongs to another gateway.
func (s *Service) ReportMeasurement(ctx context.Context, gatewayID uuid.UUID, m domain.Measurement) error {
	if err := s.checkGateway(ctx, gatewayID, m.RobotID); err != nil {
		return err
	}
	m.ReceivedAt = time.Now().UTC()
	_, err := s.repo.CreateMeasurement(ctx, m)
	return err
}

// ReportFault stores a fault the gateway gatewayID sent for one of its
// robots, with the time it arrived. A fault already stored is not stored
// again. It returns the same errors as ReportMeasurement.
func (s *Service) ReportFault(ctx context.Context, gatewayID uuid.UUID, f domain.Fault) error {
	if err := s.checkGateway(ctx, gatewayID, f.RobotID); err != nil {
		return err
	}
	f.ReceivedAt = time.Now().UTC()
	_, err := s.repo.CreateFault(ctx, f)
	return err
}

// ListMeasurements returns a robot's measurements taken from from
// (inclusive) to to (exclusive), oldest first, or domain.ErrNotFound for an
// unknown robot. The caller limits the range (domain.MaxTimeRange).
func (s *Service) ListMeasurements(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Measurement, error) {
	if _, err := s.repo.GetRobot(ctx, robotID); err != nil {
		return nil, err
	}
	return s.repo.ListMeasurements(ctx, robotID, from, to)
}

// ListFaults returns a robot's faults that occurred from from (inclusive)
// to to (exclusive), oldest first, or domain.ErrNotFound for an unknown
// robot. The caller limits the range (domain.MaxTimeRange).
func (s *Service) ListFaults(ctx context.Context, robotID uuid.UUID, from, to time.Time) ([]domain.Fault, error) {
	if _, err := s.repo.GetRobot(ctx, robotID); err != nil {
		return nil, err
	}
	return s.repo.ListFaults(ctx, robotID, from, to)
}

// checkGateway returns domain.ErrNotFound for an unknown robot and
// domain.ErrOtherGateway if the robot does not belong to gatewayID.
func (s *Service) checkGateway(ctx context.Context, gatewayID, robotID uuid.UUID) error {
	stored, err := s.repo.GetRobot(ctx, robotID)
	if err != nil {
		return err
	}
	if stored.GatewayID != gatewayID {
		return domain.ErrOtherGateway
	}
	return nil
}
