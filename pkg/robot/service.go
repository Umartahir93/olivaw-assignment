// Package robot is the service layer for robots: it discovers them from
// their gateway's scans, records when they report and answers questions
// about them.
//
// Input is validated by the handlers; this package applies business rules.
package robot

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// Repository stores robots. Lists are ordered oldest first by FirstSeenAt.
// It reports an unknown robot as domain.ErrNotFound. Any other error means
// the storage itself failed.
type Repository interface {
	CreateRobots(ctx context.Context, robots []domain.Robot) ([]domain.Robot, error)
	GetRobot(ctx context.Context, id uuid.UUID) (domain.Robot, error)
	GetRobotsBySerial(ctx context.Context, serialNumbers []string) ([]domain.Robot, error)
	ListRobots(ctx context.Context, gatewayID uuid.UUID) ([]domain.Robot, error)
	UpdateRobotLastSeen(ctx context.Context, id uuid.UUID, at time.Time) (domain.Robot, error)
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
// missing from the scan are left as they are. Reporting the same scan twice changes nothing.
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
	var newRobots []domain.Robot
	for _, r := range scanned {
		k, ok := bySerial[r.SerialNumber]
		switch {
		case ok && k.GatewayID != r.GatewayID:
			return nil, domain.ErrOtherGateway
		case !ok:
			newRobots = append(newRobots, domain.Robot{ID: r.ID, SerialNumber: r.SerialNumber, GatewayID: r.GatewayID, FirstSeenAt: now})
		}
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

// MarkSeen marks a robot as seen now, which makes it online; data from the
// robot arriving through the gateway gatewayID counts. It returns
// domain.ErrNotFound for an unknown robot and domain.ErrOtherGateway if the
// robot belongs to another gateway.
func (s *Service) MarkSeen(ctx context.Context, gatewayID, robotID uuid.UUID) (domain.Robot, error) {
	stored, err := s.repo.GetRobot(ctx, robotID)
	if err != nil {
		return domain.Robot{}, err
	}
	if stored.GatewayID != gatewayID {
		return domain.Robot{}, domain.ErrOtherGateway
	}
	return s.repo.UpdateRobotLastSeen(ctx, robotID, time.Now().UTC())
}
