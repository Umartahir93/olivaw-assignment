package gateway

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

// FakeRepository is an in-memory Repository for tests. Set Err to make
// every call fail with it.
type FakeRepository struct {
	Err      error
	Gateways []domain.Gateway
}

var _ Repository = (*FakeRepository)(nil)

func (f *FakeRepository) CreateGateway(_ context.Context, g domain.Gateway) (domain.Gateway, error) {
	if f.Err != nil {
		return domain.Gateway{}, f.Err
	}
	for _, existing := range f.Gateways {
		if existing.SerialNumber == g.SerialNumber {
			return domain.Gateway{}, domain.ErrAlreadyExists
		}
	}
	f.Gateways = append(f.Gateways, g)
	return g, nil
}

func (f *FakeRepository) GetGateway(_ context.Context, id uuid.UUID) (domain.Gateway, error) {
	if f.Err != nil {
		return domain.Gateway{}, f.Err
	}
	for _, g := range f.Gateways {
		if g.ID == id {
			return g, nil
		}
	}
	return domain.Gateway{}, domain.ErrNotFound
}

func (f *FakeRepository) ListGateways(context.Context) ([]domain.Gateway, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Gateways, nil
}

func (f *FakeRepository) TouchGateway(_ context.Context, id uuid.UUID, at time.Time) (domain.Gateway, error) {
	if f.Err != nil {
		return domain.Gateway{}, f.Err
	}
	for i := range f.Gateways {
		if f.Gateways[i].ID == id {
			f.Gateways[i].LastSeenAt = &at
			return f.Gateways[i], nil
		}
	}
	return domain.Gateway{}, domain.ErrNotFound
}
