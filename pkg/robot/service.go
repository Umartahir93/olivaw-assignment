// Package robot is the service layer for robots: it discovers them from
// their gateway's scans, records when they report and answers questions
// about them.
//
// Input is validated by the handlers; this package applies business rules.
package robot

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// Repository stores robots. Lists are ordered oldest first by FirstSeenAt.
type Repository interface {
	CreateRobots(ctx context.Context, robots []domain.Robot) ([]domain.Robot, error)
	GetRobot(ctx context.Context, id uuid.UUID) (domain.Robot, error)
	RobotsBySerial(ctx context.Context, serialNumbers []string) ([]domain.Robot, error)
	ListRobots(ctx context.Context, gatewayID uuid.UUID) ([]domain.Robot, error)
	TouchRobot(ctx context.Context, id uuid.UUID, at time.Time) (domain.Robot, error)
}

// Service handles robots.
type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() uuid.UUID
}

// NewService returns a Service that stores robots in repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, newID: uuid.New}
}

// ReportScan takes the robots a gateway's scan found, each with its serial number and the gateway's ID, and returns them as the system knows
// them, in the same order. Robots seen for the first time get an ID; known robots keep theirs; robots missing from the scan are left as they
// are. Reporting the same scan twice changes nothing.
//
// It returns domain.ErrOtherGateway if a scanned robot belongs to another
// gateway, and stores nothing in that case.
func (s *Service) ReportScan(ctx context.Context, scanned []domain.Robot) ([]domain.Robot, error) {
	serials := make([]string, len(scanned))
	for i, r := range scanned {
		serials[i] = r.SerialNumber
	}
	known, err := s.repo.RobotsBySerial(ctx, serials)
	if err != nil {
		return nil, fmt.Errorf("report scan: %w", err)
	}
	bySerial := make(map[string]domain.Robot, len(known))
	for _, r := range known {
		bySerial[r.SerialNumber] = r
	}

	now := s.now().UTC()
	var newRobots []domain.Robot
	for _, r := range scanned {
		k, ok := bySerial[r.SerialNumber]
		switch {
		case ok && k.GatewayID != r.GatewayID:
			return nil, fmt.Errorf("report scan: %s: %w", r.SerialNumber, domain.ErrOtherGateway)
		case !ok:
			newRobots = append(newRobots, domain.Robot{ID: s.newID(), SerialNumber: r.SerialNumber, GatewayID: r.GatewayID, FirstSeenAt: now})
		}
	}
	if len(newRobots) > 0 {
		created, err := s.repo.CreateRobots(ctx, newRobots)
		if err != nil {
			return nil, fmt.Errorf("report scan: %w", err)
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
	robots, err := s.repo.ListRobots(ctx, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("gateway robots: %w", err)
	}
	return robots, nil
}

// GetRobot returns one robot, or domain.ErrNotFound.
func (s *Service) GetRobot(ctx context.Context, id uuid.UUID) (domain.Robot, error) {
	r, err := s.repo.GetRobot(ctx, id)
	if err != nil {
		return domain.Robot{}, fmt.Errorf("get robot: %w", err)
	}
	return r, nil
}

// MarkSeen marks a robot as seen now, which makes it online; data from the
// robot arriving through the gateway gatewayID counts. It returns
// domain.ErrNotFound for an unknown robot and domain.ErrOtherGateway if the
// robot belongs to another gateway.
func (s *Service) MarkSeen(ctx context.Context, gatewayID, robotID uuid.UUID) (domain.Robot, error) {
	stored, err := s.repo.GetRobot(ctx, robotID)
	if err != nil {
		return domain.Robot{}, fmt.Errorf("mark seen: %w", err)
	}
	if stored.GatewayID != gatewayID {
		return domain.Robot{}, fmt.Errorf("mark seen: %w", domain.ErrOtherGateway)
	}
	seen, err := s.repo.TouchRobot(ctx, robotID, s.now().UTC())
	if err != nil {
		return domain.Robot{}, fmt.Errorf("mark seen: %w", err)
	}
	return seen, nil
}
