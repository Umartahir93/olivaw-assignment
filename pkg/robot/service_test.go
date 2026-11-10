//go:build unit

package robot

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

var (
	now      = time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	errDB    = errors.New("database down")
	gateway1 = uuid.MustParse("10000000-0000-0000-0000-000000000001")
	gateway2 = uuid.MustParse("20000000-0000-0000-0000-000000000002")
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

// robot is the robot with the n-th ID, as a scan creates it.
func robot(n int, serial string, gatewayID uuid.UUID) domain.Robot {
	return domain.Robot{
		ID:           id(n),
		SerialNumber: serial,
		GatewayID:    gatewayID,
		FirstSeenAt:  now,
	}
}

// scanned turns serial numbers into the robots a scan of gatewayID found.
func scanned(gatewayID uuid.UUID, serials ...string) []domain.Robot {
	robots := make([]domain.Robot, len(serials))
	for i, serial := range serials {
		robots[i] = domain.Robot{SerialNumber: serial, GatewayID: gatewayID}
	}
	return robots
}

// scan reports serials for gatewayID and fails the test on error.
func scan(t *testing.T, s *Service, gatewayID uuid.UUID, serials ...string) {
	t.Helper()
	if _, err := s.ReportScan(context.Background(), scanned(gatewayID, serials...)); err != nil {
		t.Fatalf("scan: %v", err)
	}
}

func TestReportScan(t *testing.T) {
	tests := []struct {
		name       string
		earlier    []string // scanned by gateway 1 first
		other      []string // scanned by gateway 2 next
		scan       []string // scanned by gateway 1 under test
		repoErr    error
		want       []domain.Robot
		wantStored []domain.Robot
		wantErr    error
	}{
		{
			name: "first scan creates every robot",
			scan: []string{"SN-1", "SN-2"},
			want: []domain.Robot{
				robot(1, "SN-1", gateway1),
				robot(2, "SN-2", gateway1),
			},
			wantStored: []domain.Robot{
				robot(1, "SN-1", gateway1),
				robot(2, "SN-2", gateway1),
			},
		},
		{
			name:    "same scan again changes nothing",
			earlier: []string{"SN-1", "SN-2"},
			scan:    []string{"SN-1", "SN-2"},
			want: []domain.Robot{
				robot(1, "SN-1", gateway1),
				robot(2, "SN-2", gateway1),
			},
			wantStored: []domain.Robot{
				robot(1, "SN-1", gateway1),
				robot(2, "SN-2", gateway1),
			},
		},
		{
			name:    "new robot joins known ones",
			earlier: []string{"SN-1"},
			scan:    []string{"SN-1", "SN-2"},
			want: []domain.Robot{
				robot(1, "SN-1", gateway1),
				robot(2, "SN-2", gateway1),
			},
			wantStored: []domain.Robot{
				robot(1, "SN-1", gateway1),
				robot(2, "SN-2", gateway1),
			},
		},
		{
			name:    "robot missing from the scan is kept",
			earlier: []string{"SN-1", "SN-2"},
			scan:    []string{"SN-1"},
			want: []domain.Robot{
				robot(1, "SN-1", gateway1),
			},
			wantStored: []domain.Robot{
				robot(1, "SN-1", gateway1),
				robot(2, "SN-2", gateway1),
			},
		},
		{
			name:       "empty scan",
			scan:       []string{},
			want:       []domain.Robot{},
			wantStored: nil,
		},
		{
			name:  "robot of another gateway is rejected",
			other: []string{"SN-9"},
			scan:  []string{"SN-1", "SN-9"},
			want:  nil,
			wantStored: []domain.Robot{
				robot(1, "SN-9", gateway2),
			},
			wantErr: domain.ErrOtherGateway,
		},
		{
			name:       "repository fails",
			scan:       []string{"SN-1"},
			repoErr:    errDB,
			want:       nil,
			wantStored: nil,
			wantErr:    errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := newTestService(repo)
			scan(t, s, gateway1, tt.earlier...)
			scan(t, s, gateway2, tt.other...)
			repo.Err = tt.repoErr

			got, err := s.ReportScan(context.Background(), scanned(gateway1, tt.scan...))

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("robots mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantStored, repo.Robots); diff != "" {
				t.Errorf("stored robots mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGatewayRobots(t *testing.T) {
	tests := []struct {
		name    string
		gateway uuid.UUID
		want    []domain.Robot
	}{
		{
			name:    "gateway with robots",
			gateway: gateway1,
			want: []domain.Robot{
				robot(1, "SN-1", gateway1),
				robot(2, "SN-2", gateway1),
			},
		},
		{
			name:    "gateway without robots",
			gateway: uuid.New(),
			want:    []domain.Robot{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestService(&FakeRepository{})
			scan(t, s, gateway1, "SN-1", "SN-2")
			scan(t, s, gateway2, "SN-9")

			got, err := s.GatewayRobots(context.Background(), tt.gateway)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("robots mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetRobot(t *testing.T) {
	tests := []struct {
		name    string
		id      uuid.UUID
		want    domain.Robot
		wantErr error
	}{
		{
			name: "known",
			id:   id(1),
			want: robot(1, "SN-1", gateway1),
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
			scan(t, s, gateway1, "SN-1")

			got, err := s.GetRobot(context.Background(), tt.id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("robot mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMarkSeen(t *testing.T) {
	seen := robot(1, "SN-1", gateway1)
	seen.LastSeenAt = ptr(now)

	tests := []struct {
		name       string
		gateway    uuid.UUID
		robot      uuid.UUID
		want       domain.Robot
		wantStored []domain.Robot
		wantErr    error
	}{
		{
			name:       "robot of this gateway",
			gateway:    gateway1,
			robot:      id(1),
			want:       seen,
			wantStored: []domain.Robot{seen},
		},
		{
			name:       "robot of another gateway",
			gateway:    gateway2,
			robot:      id(1),
			wantStored: []domain.Robot{robot(1, "SN-1", gateway1)},
			wantErr:    domain.ErrOtherGateway,
		},
		{
			name:       "unknown robot",
			gateway:    gateway1,
			robot:      uuid.New(),
			wantStored: []domain.Robot{robot(1, "SN-1", gateway1)},
			wantErr:    domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := newTestService(repo)
			scan(t, s, gateway1, "SN-1")

			got, err := s.MarkSeen(context.Background(), tt.gateway, tt.robot)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("robot mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantStored, repo.Robots); diff != "" {
				t.Errorf("stored robots mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
