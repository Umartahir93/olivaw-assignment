//go:build unit

package robot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

var (
	now   = time.Now().UTC()
	errDB = errors.New("database down")

	// approxTime treats times less than a second apart as equal, because the
	// service sets times itself with time.Now.
	approxTime = cmpopts.EquateApproxTime(time.Second)

	gateway1ID = uuid.New()
	gateway2ID = uuid.New()

	robot1ID = uuid.New() // SN-1
	robot2ID = uuid.New() // SN-2
	robot9ID = uuid.New() // SN-9
)

func ptr[T any](v T) *T { return &v }

// scanned is a robot as a gateway reports it in a scan.
func scanned(id uuid.UUID, serial string, gatewayID uuid.UUID) domain.Robot {
	return domain.Robot{
		ID:           id,
		SerialNumber: serial,
		GatewayID:    gatewayID,
	}
}

// stored is a robot as the system stores it after its first scan.
func stored(id uuid.UUID, serial string, gatewayID uuid.UUID) domain.Robot {
	r := scanned(id, serial, gatewayID)
	r.FirstSeenAt = now
	return r
}

// scan reports robots and fails the test on error.
func scan(t *testing.T, s *Service, robots ...domain.Robot) {
	t.Helper()
	if _, err := s.ReportScan(context.Background(), robots); err != nil {
		t.Fatalf("scan: %v", err)
	}
}

func TestReportScan(t *testing.T) {
	tests := []struct {
		name       string
		earlier    []domain.Robot // scanned before the scan under test
		scan       []domain.Robot // the scan under test
		repoErr    error
		want       []domain.Robot
		wantStored []domain.Robot
		wantErr    error
	}{
		{
			name: "first scan stores every robot",
			scan: []domain.Robot{
				scanned(robot1ID, "SN-1", gateway1ID),
				scanned(robot2ID, "SN-2", gateway1ID),
			},
			want: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
				stored(robot2ID, "SN-2", gateway1ID),
			},
			wantStored: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
				stored(robot2ID, "SN-2", gateway1ID),
			},
		},
		{
			name: "known robot keeps its stored ID",
			earlier: []domain.Robot{
				scanned(robot1ID, "SN-1", gateway1ID),
			},
			scan: []domain.Robot{
				scanned(uuid.New(), "SN-1", gateway1ID),
			},
			want: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
			},
			wantStored: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
			},
		},
		{
			name: "new robot joins known ones",
			earlier: []domain.Robot{
				scanned(robot1ID, "SN-1", gateway1ID),
			},
			scan: []domain.Robot{
				scanned(robot1ID, "SN-1", gateway1ID),
				scanned(robot2ID, "SN-2", gateway1ID),
			},
			want: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
				stored(robot2ID, "SN-2", gateway1ID),
			},
			wantStored: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
				stored(robot2ID, "SN-2", gateway1ID),
			},
		},
		{
			name: "robot missing from the scan is kept",
			earlier: []domain.Robot{
				scanned(robot1ID, "SN-1", gateway1ID),
				scanned(robot2ID, "SN-2", gateway1ID),
			},
			scan: []domain.Robot{
				scanned(robot1ID, "SN-1", gateway1ID),
			},
			want: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
			},
			wantStored: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
				stored(robot2ID, "SN-2", gateway1ID),
			},
		},
		{
			name:       "empty scan",
			scan:       []domain.Robot{},
			want:       []domain.Robot{},
			wantStored: nil,
		},
		{
			name: "robot of another gateway is rejected",
			earlier: []domain.Robot{
				scanned(robot9ID, "SN-9", gateway2ID),
			},
			scan: []domain.Robot{
				scanned(robot1ID, "SN-1", gateway1ID),
				scanned(robot9ID, "SN-9", gateway1ID),
			},
			want: nil,
			wantStored: []domain.Robot{
				stored(robot9ID, "SN-9", gateway2ID),
			},
			wantErr: domain.ErrOtherGateway,
		},
		{
			name: "repository fails",
			scan: []domain.Robot{
				scanned(robot1ID, "SN-1", gateway1ID),
			},
			repoErr:    errDB,
			want:       nil,
			wantStored: nil,
			wantErr:    errDB,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := NewService(repo)
			scan(t, s, tt.earlier...)
			repo.Err = tt.repoErr

			got, err := s.ReportScan(context.Background(), tt.scan)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("robots mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantStored, repo.Robots, approxTime); diff != "" {
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
			gateway: gateway1ID,
			want: []domain.Robot{
				stored(robot1ID, "SN-1", gateway1ID),
				stored(robot2ID, "SN-2", gateway1ID),
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
			s := NewService(&FakeRepository{})
			scan(t, s,
				scanned(robot1ID, "SN-1", gateway1ID),
				scanned(robot2ID, "SN-2", gateway1ID),
				scanned(robot9ID, "SN-9", gateway2ID),
			)

			got, err := s.GatewayRobots(context.Background(), tt.gateway)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
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
			id:   robot1ID,
			want: stored(robot1ID, "SN-1", gateway1ID),
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
			scan(t, s, scanned(robot1ID, "SN-1", gateway1ID))

			got, err := s.GetRobot(context.Background(), tt.id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("robot mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMarkSeen(t *testing.T) {
	seen := stored(robot1ID, "SN-1", gateway1ID)
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
			gateway:    gateway1ID,
			robot:      robot1ID,
			want:       seen,
			wantStored: []domain.Robot{seen},
		},
		{
			name:       "robot of another gateway",
			gateway:    gateway2ID,
			robot:      robot1ID,
			wantStored: []domain.Robot{stored(robot1ID, "SN-1", gateway1ID)},
			wantErr:    domain.ErrOtherGateway,
		},
		{
			name:       "unknown robot",
			gateway:    gateway1ID,
			robot:      uuid.New(),
			wantStored: []domain.Robot{stored(robot1ID, "SN-1", gateway1ID)},
			wantErr:    domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeRepository{}
			s := NewService(repo)
			scan(t, s, scanned(robot1ID, "SN-1", gateway1ID))

			got, err := s.MarkSeen(context.Background(), tt.gateway, tt.robot)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("robot mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantStored, repo.Robots, approxTime); diff != "" {
				t.Errorf("stored robots mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
