// Package repository stores robots. It is a stub: every method returns
// ErrUnimplemented until storage is built.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot"
	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

// ErrUnimplemented is returned by every method until storage is built.
var ErrUnimplemented = errors.New("robot repository: not implemented")

// Repository implements robot.Repository.
type Repository struct{}

var _ robot.Repository = (*Repository)(nil)

// New returns a Repository.
func New() *Repository {
	return &Repository{}
}

// CreateRobots is not implemented yet.
func (r *Repository) CreateRobots(context.Context, []domain.Robot) ([]domain.Robot, error) {
	return nil, ErrUnimplemented
}

// GetRobot is not implemented yet.
func (r *Repository) GetRobot(context.Context, uuid.UUID) (domain.Robot, error) {
	return domain.Robot{}, ErrUnimplemented
}

// GetRobotsBySerial is not implemented yet.
func (r *Repository) GetRobotsBySerial(context.Context, []string) ([]domain.Robot, error) {
	return nil, ErrUnimplemented
}

// ListRobots is not implemented yet.
func (r *Repository) ListRobots(context.Context, uuid.UUID) ([]domain.Robot, error) {
	return nil, ErrUnimplemented
}

// UpdateRobotLastSeen is not implemented yet.
func (r *Repository) UpdateRobotLastSeen(context.Context, uuid.UUID, time.Time) (domain.Robot, error) {
	return domain.Robot{}, ErrUnimplemented
}
