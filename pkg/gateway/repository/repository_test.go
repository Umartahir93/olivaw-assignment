//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	"github.com/umartahir93/olivaw-assignment/pkg/gateway/repository"
	postgrestest "github.com/umartahir93/olivaw-assignment/test/postgres"
)

var (
	// Times have microsecond precision, like the database.
	registeredAt = time.Date(2026, 11, 10, 10, 0, 0, 123456000, time.UTC)
	seenAt       = registeredAt.Add(time.Hour)

	gateway1 = domain.Gateway{
		ID:           uuid.New(),
		SerialNumber: "GW-1",
		Location:     "Munich HQ",
		Name:         ptr("Main hall"),
		RegisteredAt: registeredAt,
	}
	gateway2 = domain.Gateway{
		ID:           uuid.New(),
		SerialNumber: "GW-2",
		Location:     "Berlin",
		RegisteredAt: registeredAt.Add(time.Minute),
	}

	// equateEmpty treats nil and empty slices as equal.
	equateEmpty = cmpopts.EquateEmpty()
)

func ptr[T any](v T) *T { return &v }

// withLastSeen returns g as seen at at.
func withLastSeen(g domain.Gateway, at time.Time) domain.Gateway {
	g.LastSeenAt = &at
	return g
}

// newRepository returns a repository on an empty test database holding
// gateways, and closes it when the test ends.
func newRepository(t *testing.T, gateways ...domain.Gateway) *repository.Repository {
	t.Helper()
	db := postgrestest.Open(t)
	postgrestest.Reset(t, db)

	repo, err := repository.New(context.Background(), db)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	t.Cleanup(func() {
		if err := repo.Close(); err != nil {
			t.Errorf("close repository: %v", err)
		}
	})

	for _, g := range gateways {
		if _, err := repo.CreateGateway(context.Background(), g); err != nil {
			t.Fatalf("create gateway: %v", err)
		}
	}
	return repo
}

func TestCreateGateway(t *testing.T) {
	tests := []struct {
		name       string
		existing   []domain.Gateway
		gateway    domain.Gateway
		want       domain.Gateway
		wantStored []domain.Gateway
		wantErr    error
	}{
		{
			name:       "new gateway",
			gateway:    gateway1,
			want:       gateway1,
			wantStored: []domain.Gateway{gateway1},
		},
		{
			name:       "gateway without a name",
			gateway:    gateway2,
			want:       gateway2,
			wantStored: []domain.Gateway{gateway2},
		},
		{
			name:       "gateway that was seen",
			gateway:    withLastSeen(gateway1, seenAt),
			want:       withLastSeen(gateway1, seenAt),
			wantStored: []domain.Gateway{withLastSeen(gateway1, seenAt)},
		},
		{
			name: "serial number taken",
			existing: []domain.Gateway{
				gateway1,
			},
			gateway: domain.Gateway{
				ID:           uuid.New(),
				SerialNumber: "GW-1",
				Location:     "Hamburg",
				RegisteredAt: registeredAt,
			},
			wantStored: []domain.Gateway{gateway1},
			wantErr:    domain.ErrAlreadyExists,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newRepository(t, tt.existing...)

			got, err := repo.CreateGateway(ctx, tt.gateway)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("gateway mismatch (-want +got):\n%s", diff)
			}
			stored, err := repo.ListGateways(ctx)
			if err != nil {
				t.Fatalf("list gateways: %v", err)
			}
			if diff := cmp.Diff(tt.wantStored, stored, equateEmpty); diff != "" {
				t.Errorf("stored gateways mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetGateway(t *testing.T) {
	tests := []struct {
		name    string
		id      uuid.UUID
		want    domain.Gateway
		wantErr error
	}{
		{
			name: "stored gateway",
			id:   gateway1.ID,
			want: gateway1,
		},
		{
			name:    "unknown gateway",
			id:      uuid.New(),
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepository(t, gateway1, gateway2)

			got, err := repo.GetGateway(context.Background(), tt.id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("gateway mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListGateways(t *testing.T) {
	tests := []struct {
		name     string
		existing []domain.Gateway
		want     []domain.Gateway
	}{
		{
			name:     "no gateways",
			existing: nil,
			want:     nil,
		},
		{
			name: "oldest first, whatever the insert order",
			existing: []domain.Gateway{
				gateway2,
				gateway1,
			},
			want: []domain.Gateway{
				gateway1,
				gateway2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepository(t, tt.existing...)

			got, err := repo.ListGateways(context.Background())

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, equateEmpty); diff != "" {
				t.Errorf("gateways mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateGatewayLastSeen(t *testing.T) {
	tests := []struct {
		name       string
		id         uuid.UUID
		at         time.Time
		want       domain.Gateway
		wantStored []domain.Gateway
		wantErr    error
	}{
		{
			name: "stored gateway",
			id:   gateway1.ID,
			at:   seenAt,
			want: withLastSeen(gateway1, seenAt),
			wantStored: []domain.Gateway{
				withLastSeen(gateway1, seenAt),
				gateway2,
			},
		},
		{
			name: "unknown gateway",
			id:   uuid.New(),
			at:   seenAt,
			wantStored: []domain.Gateway{
				gateway1,
				gateway2,
			},
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newRepository(t, gateway1, gateway2)

			got, err := repo.UpdateGatewayLastSeen(ctx, tt.id, tt.at)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("gateway mismatch (-want +got):\n%s", diff)
			}
			stored, err := repo.ListGateways(ctx)
			if err != nil {
				t.Fatalf("list gateways: %v", err)
			}
			if diff := cmp.Diff(tt.wantStored, stored); diff != "" {
				t.Errorf("stored gateways mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
