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

	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	gatewayrepository "github.com/umartahir93/olivaw-assignment/pkg/gateway/repository"
	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
	"github.com/umartahir93/olivaw-assignment/pkg/robot/repository"
	postgrestest "github.com/umartahir93/olivaw-assignment/test/postgres"
)

var (
	// Times have microsecond precision, like the database.
	firstSeenAt = time.Date(2026, 11, 10, 10, 0, 0, 123456000, time.UTC)
	seenAt      = firstSeenAt.Add(time.Hour)

	gateway1 = gatewaydomain.Gateway{
		ID:           uuid.New(),
		SerialNumber: "GW-1",
		Location:     "Munich HQ",
		RegisteredAt: firstSeenAt,
	}
	gateway2 = gatewaydomain.Gateway{
		ID:           uuid.New(),
		SerialNumber: "GW-2",
		Location:     "Berlin",
		RegisteredAt: firstSeenAt,
	}

	robot1 = domain.Robot{
		ID:           uuid.New(),
		SerialNumber: "SN-1",
		GatewayID:    gateway1.ID,
		FirstSeenAt:  firstSeenAt,
	}
	robot2 = domain.Robot{
		ID:           uuid.New(),
		SerialNumber: "SN-2",
		GatewayID:    gateway1.ID,
		FirstSeenAt:  firstSeenAt.Add(time.Minute),
	}
	robot9 = domain.Robot{
		ID:           uuid.New(),
		SerialNumber: "SN-9",
		GatewayID:    gateway2.ID,
		FirstSeenAt:  firstSeenAt.Add(2 * time.Minute),
	}
	// robotOfUnknownGateway belongs to a gateway that is not stored.
	robotOfUnknownGateway = domain.Robot{
		ID:           uuid.New(),
		SerialNumber: "SN-X",
		GatewayID:    uuid.New(),
		FirstSeenAt:  firstSeenAt,
	}

	// equateEmpty treats nil and empty slices as equal.
	equateEmpty = cmpopts.EquateEmpty()
)

// withLastSeen returns r as seen at at.
func withLastSeen(r domain.Robot, at time.Time) domain.Robot {
	r.LastSeenAt = &at
	return r
}

// newRepository returns a repository on an empty test database holding
// gateway1, gateway2 and robots, and closes it when the test ends.
func newRepository(t *testing.T, robots ...domain.Robot) *repository.Repository {
	t.Helper()
	ctx := context.Background()
	db := postgrestest.Open(t)
	postgrestest.Reset(t, db)

	gateways, err := gatewayrepository.New(ctx, db)
	if err != nil {
		t.Fatalf("new gateway repository: %v", err)
	}
	t.Cleanup(func() {
		if err := gateways.Close(); err != nil {
			t.Errorf("close gateway repository: %v", err)
		}
	})
	for _, g := range []gatewaydomain.Gateway{gateway1, gateway2} {
		if _, err := gateways.CreateGateway(ctx, g); err != nil {
			t.Fatalf("create gateway: %v", err)
		}
	}

	repo, err := repository.New(ctx, db)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}
	t.Cleanup(func() {
		if err := repo.Close(); err != nil {
			t.Errorf("close repository: %v", err)
		}
	})

	if len(robots) > 0 {
		if _, err := repo.CreateRobots(ctx, robots); err != nil {
			t.Fatalf("create robots: %v", err)
		}
	}
	return repo
}

func TestCreateRobots(t *testing.T) {
	tests := []struct {
		name       string
		robots     []domain.Robot
		want       []domain.Robot
		wantStored []domain.Robot
		wantErr    bool
	}{
		{
			name: "new robots",
			robots: []domain.Robot{
				robot1,
				robot2,
			},
			want: []domain.Robot{
				robot1,
				robot2,
			},
			wantStored: []domain.Robot{
				robot1,
				robot2,
			},
		},
		{
			name: "robot that was seen",
			robots: []domain.Robot{
				withLastSeen(robot1, seenAt),
			},
			want: []domain.Robot{
				withLastSeen(robot1, seenAt),
			},
			wantStored: []domain.Robot{
				withLastSeen(robot1, seenAt),
			},
		},
		{
			name: "all or nothing: a robot of an unknown gateway stores none",
			robots: []domain.Robot{
				robot1,
				robotOfUnknownGateway,
			},
			want:       nil,
			wantStored: nil,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newRepository(t)

			got, err := repo.CreateRobots(ctx, tt.robots)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want an error: %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, equateEmpty); diff != "" {
				t.Errorf("robots mismatch (-want +got):\n%s", diff)
			}
			stored, err := repo.ListRobots(ctx, gateway1.ID)
			if err != nil {
				t.Fatalf("list robots: %v", err)
			}
			if diff := cmp.Diff(tt.wantStored, stored, equateEmpty); diff != "" {
				t.Errorf("stored robots mismatch (-want +got):\n%s", diff)
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
			name: "stored robot",
			id:   robot1.ID,
			want: robot1,
		},
		{
			name:    "unknown robot",
			id:      uuid.New(),
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepository(t, robot1, robot2)

			got, err := repo.GetRobot(context.Background(), tt.id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("robot mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetRobotsBySerial(t *testing.T) {
	tests := []struct {
		name    string
		serials []string
		want    []domain.Robot
	}{
		{
			name:    "robots of any gateway",
			serials: []string{"SN-1", "SN-9"},
			want: []domain.Robot{
				robot1,
				robot9,
			},
		},
		{
			name:    "unknown serial numbers are left out",
			serials: []string{"SN-2", "SN-404"},
			want: []domain.Robot{
				robot2,
			},
		},
		{
			name:    "no serial numbers",
			serials: []string{},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepository(t, robot1, robot2, robot9)

			got, err := repo.GetRobotsBySerial(context.Background(), tt.serials)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, equateEmpty); diff != "" {
				t.Errorf("robots mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListRobots(t *testing.T) {
	tests := []struct {
		name    string
		gateway uuid.UUID
		want    []domain.Robot
	}{
		{
			name:    "only the gateway's robots, oldest first",
			gateway: gateway1.ID,
			want: []domain.Robot{
				robot1,
				robot2,
			},
		},
		{
			name:    "other gateway",
			gateway: gateway2.ID,
			want: []domain.Robot{
				robot9,
			},
		},
		{
			name:    "unknown gateway",
			gateway: uuid.New(),
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRepository(t, robot2, robot9, robot1)

			got, err := repo.ListRobots(context.Background(), tt.gateway)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, equateEmpty); diff != "" {
				t.Errorf("robots mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateRobotLastSeen(t *testing.T) {
	tests := []struct {
		name       string
		id         uuid.UUID
		at         time.Time
		want       domain.Robot
		wantStored []domain.Robot
		wantErr    error
	}{
		{
			name: "stored robot",
			id:   robot1.ID,
			at:   seenAt,
			want: withLastSeen(robot1, seenAt),
			wantStored: []domain.Robot{
				withLastSeen(robot1, seenAt),
				robot2,
			},
		},
		{
			name: "unknown robot",
			id:   uuid.New(),
			at:   seenAt,
			wantStored: []domain.Robot{
				robot1,
				robot2,
			},
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newRepository(t, robot1, robot2)

			got, err := repo.UpdateRobotLastSeen(ctx, tt.id, tt.at)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("robot mismatch (-want +got):\n%s", diff)
			}
			stored, err := repo.ListRobots(ctx, gateway1.ID)
			if err != nil {
				t.Fatalf("list robots: %v", err)
			}
			if diff := cmp.Diff(tt.wantStored, stored); diff != "" {
				t.Errorf("stored robots mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
