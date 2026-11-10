//go:build unit

package gateway

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
)

var (
	now   = time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	errDB = errors.New("database down")
)

// id returns the n-th ID the test service hands out.
func id(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", n))
}

func ptr[T any](v T) *T { return &v }

// newTestService returns a service with a fixed clock that hands out
// id(1), id(2), … as IDs.
func newTestService(repo *FakeRepository) *Service {
	s := NewService(repo)
	s.now = func() time.Time { return now }
	n := 0
	s.newID = func() uuid.UUID {
		n++
		return id(n)
	}
	return s
}

// registered is the gateway GW-n as registerGateways stores it.
func registered(n int) domain.Gateway {
	return domain.Gateway{
		ID:           id(n),
		SerialNumber: fmt.Sprintf("GW-%d", n),
		Location:     "Munich HQ",
		RegisteredAt: now,
	}
}

// registerGateways registers GW-1 … GW-count.
func registerGateways(t *testing.T, s *Service, count int) {
	t.Helper()
	for n := 1; n <= count; n++ {
		g := domain.Gateway{SerialNumber: fmt.Sprintf("GW-%d", n), Location: "Munich HQ"}
		if _, err := s.RegisterGateway(context.Background(), g); err != nil {
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
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
				Name:         ptr("Main hall"),
			},
			want: domain.Gateway{
				ID:           id(2),
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
				Name:         ptr("Main hall"),
				RegisteredAt: now,
			},
		},
		{
			name: "ID and times from the caller are replaced",
			gateway: domain.Gateway{
				ID:           uuid.New(),
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
				RegisteredAt: now.Add(-time.Hour),
				LastSeenAt:   ptr(now.Add(-time.Hour)),
			},
			want: domain.Gateway{
				ID:           id(2),
				SerialNumber: "GW-NEW",
				Location:     "Munich HQ",
				RegisteredAt: now,
			},
		},
		{
			name: "serial number taken",
			gateway: domain.Gateway{
				SerialNumber: "GW-1",
				Location:     "Munich HQ",
			},
			wantErr: domain.ErrAlreadyExists,
		},
		{
			name: "repository fails",
			gateway: domain.Gateway{
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
			s := newTestService(repo)
			registerGateways(t, s, 1)
			repo.Err = tt.repoErr

			got, err := s.RegisterGateway(context.Background(), tt.gateway)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
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
			id:   id(1),
			want: registered(1),
		},
		{
			name:    "unknown",
			id:      uuid.New(),
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestService(&FakeRepository{})
			registerGateways(t, s, 1)

			got, err := s.GetGateway(context.Background(), tt.id)

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
		name    string
		count   int
		repoErr error
		want    []domain.Gateway
		wantErr error
	}{
		{
			name:  "no gateways",
			count: 0,
			want:  nil,
		},
		{
			name:  "all gateways, oldest first",
			count: 3,
			want: []domain.Gateway{
				registered(1),
				registered(2),
				registered(3),
			},
		},
		{
			name:    "repository fails",
			count:   1,
			repoErr: errDB,
			wantErr: errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := newTestService(repo)
			registerGateways(t, s, tt.count)
			repo.Err = tt.repoErr

			got, err := s.ListGateways(context.Background())

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("gateways mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMarkSeen(t *testing.T) {
	seen := registered(1)
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
			id:         id(1),
			want:       seen,
			wantStored: []domain.Gateway{seen},
		},
		{
			name:       "unregistered gateway",
			id:         uuid.New(),
			wantStored: []domain.Gateway{registered(1)},
			wantErr:    domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := newTestService(repo)
			registerGateways(t, s, 1)

			got, err := s.MarkSeen(context.Background(), tt.id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("gateway mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantStored, repo.Gateways); diff != "" {
				t.Errorf("stored gateways mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
