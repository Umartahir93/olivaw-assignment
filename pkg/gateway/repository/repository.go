// Package repository stores gateways. It is a stub: every method returns
// ErrUnimplemented until storage is built.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway"
	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

// ErrUnimplemented is returned by every method until storage is built.
var ErrUnimplemented = errors.New("gateway repository: not implemented")

// Repository implements gateway.Repository.
type Repository struct{}

var _ gateway.Repository = (*Repository)(nil)

// New returns a Repository.
func New() *Repository {
	return &Repository{}
}

func (r *Repository) CreateGateway(context.Context, domain.Gateway) (domain.Gateway, error) {
	return domain.Gateway{}, ErrUnimplemented
}

func (r *Repository) GetGateway(context.Context, uuid.UUID) (domain.Gateway, error) {
	return domain.Gateway{}, ErrUnimplemented
}

func (r *Repository) ListGateways(context.Context) ([]domain.Gateway, error) {
	return nil, ErrUnimplemented
}

func (r *Repository) TouchGateway(context.Context, uuid.UUID, time.Time) (domain.Gateway, error) {
	return domain.Gateway{}, ErrUnimplemented
}
