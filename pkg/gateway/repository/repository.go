// Package repository stores gateways in PostgreSQL.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/umartahir93/olivaw-assignment/utils"

	gatewayservice "github.com/umartahir93/olivaw-assignment/pkg/gateway"
	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

// Queries. Every query returns gatewayColumns, in that order.
const (
	gatewayColumns = `id, serial_number, location, name, registered_at, last_seen_at`

	insertGatewaySQL = `
		INSERT INTO gateways (id, serial_number, location, name, registered_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	selectGatewayByIDSQL = `
		SELECT ` + gatewayColumns + `
		FROM gateways
		WHERE id = $1`

	selectGatewaysSQL = `
		SELECT ` + gatewayColumns + `
		FROM gateways
		ORDER BY registered_at, id`

	updateGatewayLastSeenSQL = `
		UPDATE gateways
		SET last_seen_at = $2
		WHERE id = $1`
)

// Repository implements gateway.Repository on PostgreSQL with statements
// prepared on the database server.
type Repository struct {
	insertGateway         *sqlx.Stmt
	selectGatewayByID     *sqlx.Stmt
	selectGateways        *sqlx.Stmt
	updateGatewayLastSeen *sqlx.Stmt
}

var _ gatewayservice.Repository = (*Repository)(nil)

// New prepares the repository's statements on db. Close releases them.
func New(ctx context.Context, db *sqlx.DB) (*Repository, error) {
	r := &Repository{}

	statements := []struct {
		stmt  **sqlx.Stmt
		query string
	}{
		{&r.insertGateway, insertGatewaySQL},
		{&r.selectGatewayByID, selectGatewayByIDSQL},
		{&r.selectGateways, selectGatewaysSQL},
		{&r.updateGatewayLastSeen, updateGatewayLastSeenSQL},
	}

	for _, s := range statements {
		stmt, err := db.PreparexContext(ctx, s.query)
		if err != nil {
			_ = r.Close()
			return nil, err
		}
		*s.stmt = stmt
	}

	return r, nil
}

// Close releases the prepared statements.
func (r *Repository) Close() error {
	var errs []error

	for _, stmt := range []*sqlx.Stmt{
		r.insertGateway,
		r.selectGatewayByID,
		r.selectGateways,
		r.updateGatewayLastSeen,
	} {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// CreateGateway stores g and returns it as stored. It returns
// domain.ErrAlreadyExists if the serial number is taken.
func (r *Repository) CreateGateway(ctx context.Context, g domain.Gateway) (domain.Gateway, error) {
	gatewayDB := gatewayFromDomain(g)

	_, err := r.insertGateway.ExecContext(
		ctx,
		gatewayDB.ID,
		gatewayDB.SerialNumber,
		gatewayDB.Location,
		gatewayDB.Name,
		gatewayDB.RegisteredAt,
		gatewayDB.LastSeenAt,
	)
	if err != nil {
		switch {
		case utils.IsUniqueViolation(err):
			return domain.Gateway{}, domain.ErrAlreadyExists
		default:
			return domain.Gateway{}, err
		}
	}

	return gatewayDB.toDomain(), nil
}

// GetGateway returns the gateway with this ID, or domain.ErrNotFound.
func (r *Repository) GetGateway(ctx context.Context, id uuid.UUID) (domain.Gateway, error) {
	var gatewayDB gateway
	err := r.selectGatewayByID.GetContext(ctx, &gatewayDB, id)
	switch {
	case utils.IsNoRows(err):
		return domain.Gateway{}, domain.ErrNotFound
	case err != nil:
		return domain.Gateway{}, err
	}

	return gatewayDB.toDomain(), nil
}

// ListGateways returns all gateways, oldest first.
func (r *Repository) ListGateways(ctx context.Context) ([]domain.Gateway, error) {
	var rows []gateway

	if err := r.selectGateways.SelectContext(ctx, &rows); err != nil {
		return nil, err
	}

	gateways := make([]domain.Gateway, 0, len(rows))
	for _, row := range rows {
		gateways = append(gateways, row.toDomain())
	}

	return gateways, nil
}

// UpdateGatewayLastSeen sets the gateway's LastSeenAt to at and returns the
// gateway, or domain.ErrNotFound.
func (r *Repository) UpdateGatewayLastSeen(ctx context.Context, id uuid.UUID, at time.Time) (domain.Gateway, error) {
	result, err := r.updateGatewayLastSeen.ExecContext(ctx, id, at)
	if err != nil {
		return domain.Gateway{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Gateway{}, err
	}
	if affected == 0 {
		return domain.Gateway{}, domain.ErrNotFound
	}

	var out gateway
	err = r.selectGatewayByID.GetContext(ctx, &out, id)
	switch {
	case utils.IsNoRows(err):
		return domain.Gateway{}, domain.ErrNotFound
	case err != nil:
		return domain.Gateway{}, err
	}

	return out.toDomain(), nil
}
