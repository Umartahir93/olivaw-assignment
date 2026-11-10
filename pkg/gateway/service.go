package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

// Repository stores gateways. Lists are ordered oldest first by RegisteredAt.
type Repository interface {
	CreateGateway(ctx context.Context, g domain.Gateway) (domain.Gateway, error)
	GetGateway(ctx context.Context, id uuid.UUID) (domain.Gateway, error)
	ListGateways(ctx context.Context) ([]domain.Gateway, error)
	TouchGateway(ctx context.Context, id uuid.UUID, at time.Time) (domain.Gateway, error)
}

// Service handles gateways.
type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() uuid.UUID
}

// NewService returns a Service that stores gateways in repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, newID: uuid.New}
}

// RegisterGateway registers a gateway from its serial number, location and name, and assigns its ID and registration time. It returns domain.ErrAlreadyExists if the serial number is taken.
func (s *Service) RegisterGateway(ctx context.Context, g domain.Gateway) (domain.Gateway, error) {
	g.ID = s.newID()
	g.RegisteredAt = s.now().UTC()
	g.LastSeenAt = nil
	created, err := s.repo.CreateGateway(ctx, g)
	if err != nil {
		return domain.Gateway{}, fmt.Errorf("register gateway: %w", err)
	}
	return created, nil
}

// GetGateway returns one gateway, or domain.ErrNotFound.
func (s *Service) GetGateway(ctx context.Context, id uuid.UUID) (domain.Gateway, error) {
	g, err := s.repo.GetGateway(ctx, id)
	if err != nil {
		return domain.Gateway{}, fmt.Errorf("get gateway: %w", err)
	}
	return g, nil
}

// ListGateways returns all gateways, oldest first. For the sake of simplicity there is no pagination.
func (s *Service) ListGateways(ctx context.Context) ([]domain.Gateway, error) {
	gateways, err := s.repo.ListGateways(ctx)
	if err != nil {
		return nil, fmt.Errorf("list gateways: %w", err)
	}
	return gateways, nil
}

// MarkSeen marks a gateway as seen now, which makes it online; any call from a gateway counts. It returns domain.ErrNotFound if the gateway is
// not registered.
func (s *Service) MarkSeen(ctx context.Context, id uuid.UUID) (domain.Gateway, error) {
	seen, err := s.repo.TouchGateway(ctx, id, s.now().UTC())
	if err != nil {
		return domain.Gateway{}, fmt.Errorf("mark seen: %w", err)
	}
	return seen, nil
}
