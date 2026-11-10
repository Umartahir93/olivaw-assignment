//go:build unit

package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

var (
	now   = time.Now().UTC()
	errDB = errors.New("database down")

	// approxTime treats times less than a second apart as equal, because the
	// service sets times itself with time.Now.
	approxTime = cmpopts.EquateApproxTime(time.Second)

	gateway1ID   = uuid.New()
	gateway2ID   = uuid.New()
	gateway3ID   = uuid.New()
	newGatewayID = uuid.New()

	// gateway1 … gateway3 are gateways as stored after registration.
	gateway1 = domain.Gateway{ID: gateway1ID, SerialNumber: "GW-1", Location: "Munich HQ", RegisteredAt: now}
	gateway2 = domain.Gateway{ID: gateway2ID, SerialNumber: "GW-2", Location: "Munich HQ", RegisteredAt: now}
	gateway3 = domain.Gateway{ID: gateway3ID, SerialNumber: "GW-3", Location: "Munich HQ", RegisteredAt: now}
)

func ptr[T any](v T) *T { return &v }

// register registers gateways through the service.
func register(t *testing.T, s *Service, gateways ...domain.Gateway) {
	t.Helper()
	for _, g := range gateways {
		if _, err := s.RegisterGateway(context.Background(), domain.Gateway{ID: g.ID, SerialNumber: g.SerialNumber, Location: g.Location}); err != nil {
			t.Fatalf("register gateway: %v", err)
		}
	}
}

func TestRegisterGateway(t *testing.T) {
	tests := []struct {
		name    string
		gateway domain.Gateway
		repoErr error
		want    domain.Gateway
		wantErr error
	}{
		{
			name: "new gateway",
			gateway: domain.Gateway{
				ID:           newGatewayID,
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
				Name:         ptr("Main hall"),
			},
			want: domain.Gateway{
				ID:           newGatewayID,
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
				Name:         ptr("Main hall"),
				RegisteredAt: now,
			},
		},
		{
			name: "times from the caller are replaced",
			gateway: domain.Gateway{
				ID:           newGatewayID,
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
				RegisteredAt: now.Add(-time.Hour),
				LastSeenAt:   ptr(now.Add(-time.Hour)),
			},
			want: domain.Gateway{
				ID:           newGatewayID,
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
				RegisteredAt: now,
			},
		},
		{
			name: "serial number taken",
			gateway: domain.Gateway{
				ID:           newGatewayID,
				SerialNumber: "GW-1",
				Location:     "Munich HQ",
			},
			wantErr: domain.ErrAlreadyExists,
		},
		{
			name: "repository fails",
			gateway: domain.Gateway{
				ID:           newGatewayID,
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
			},
			repoErr: errDB,
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := NewService(repo)
			register(t, s, gateway1)
			repo.Err = tt.repoErr

			got, err := s.RegisterGateway(context.Background(), tt.gateway)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("gateway mismatch (-want +got):\n%s", diff)
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
			name: "registered",
			id:   gateway1ID,
			want: gateway1,
		},
		{
			name:    "unknown",
			id:      uuid.New(),
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewService(&FakeRepository{})
			register(t, s, gateway1)

			got, err := s.GetGateway(context.Background(), tt.id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("gateway mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListGateways(t *testing.T) {
	tests := []struct {
		name       string
		registered []domain.Gateway
		repoErr    error
		want       []domain.Gateway
		wantErr    error
	}{
		{
			name:       "no gateways",
			registered: nil,
			want:       nil,
		},
		{
			name: "all gateways, oldest first",
			registered: []domain.Gateway{
				gateway1,
				gateway2,
				gateway3,
			},
			want: []domain.Gateway{
				gateway1,
				gateway2,
				gateway3,
			},
		},
		{
			name: "repository fails",
			registered: []domain.Gateway{
				gateway1,
			},
			repoErr: errDB,
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := NewService(repo)
			register(t, s, tt.registered...)
			repo.Err = tt.repoErr

			got, err := s.ListGateways(context.Background())

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("gateways mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMarkSeen(t *testing.T) {
	seen := gateway1
	seen.LastSeenAt = ptr(now)

	tests := []struct {
		name       string
		id         uuid.UUID
		want       domain.Gateway
		wantStored []domain.Gateway
		wantErr    error
	}{
		{
			name:       "registered gateway",
			id:         gateway1ID,
			want:       seen,
			wantStored: []domain.Gateway{seen},
		},
		{
			name:       "unregistered gateway",
			id:         uuid.New(),
			wantStored: []domain.Gateway{gateway1},
			wantErr:    domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := NewService(repo)
			register(t, s, gateway1)

			got, err := s.MarkSeen(context.Background(), tt.id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("gateway mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantStored, repo.Gateways, approxTime); diff != "" {
				t.Errorf("stored gateways mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
