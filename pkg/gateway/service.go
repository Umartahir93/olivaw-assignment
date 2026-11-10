// Package gateway is the service layer for gateways: it registers them,
// marks them as seen and answers questions about them.
package gateway

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

// Repository stores gateways. Lists are ordered oldest first by RegisteredAt.
// It reports business conditions as domain errors: domain.ErrAlreadyExists for a taken serial number, domain.ErrNotFound for an unknown
// gateway. Any other error means the storage itself failed.
type Repository interface {
	CreateGateway(ctx context.Context, g domain.Gateway) (domain.Gateway, error)
	GetGateway(ctx context.Context, id uuid.UUID) (domain.Gateway, error)
	ListGateways(ctx context.Context) ([]domain.Gateway, error)
	UpdateGatewayLastSeen(ctx context.Context, id uuid.UUID, at time.Time) (domain.Gateway, error)
}

// Service handles gateways.
type Service struct {
	repo Repository
}

// NewService returns a Service that stores gateways in repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// RegisterGateway registers a gateway with its ID, serial number, location and name, registered now and not seen yet. It returns
// domain.ErrAlreadyExists if the serial number is taken.
func (s *Service) RegisterGateway(ctx context.Context, g domain.Gateway) (domain.Gateway, error) {
	g.RegisteredAt = time.Now().UTC()
	g.LastSeenAt = nil
	return s.repo.CreateGateway(ctx, g)
}

// GetGateway returns one gateway, or domain.ErrNotFound.
func (s *Service) GetGateway(ctx context.Context, id uuid.UUID) (domain.Gateway, error) {
	return s.repo.GetGateway(ctx, id)
}

// ListGateways returns all gateways, oldest first. For the sake of simplicity there is no pagination.
func (s *Service) ListGateways(ctx context.Context) ([]domain.Gateway, error) {
	return s.repo.ListGateways(ctx)
}

// MarkSeen marks a gateway as seen now, which makes it online; any call from a gateway counts. It returns domain.ErrNotFound if the gateway is
// not registered.
func (s *Service) MarkSeen(ctx context.Context, id uuid.UUID) (domain.Gateway, error) {
	return s.repo.UpdateGatewayLastSeen(ctx, id, time.Now().UTC())
}
