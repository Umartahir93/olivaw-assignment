//go:build unit

package robot_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"

	"github.com/umartahir93/olivaw-assignment/pkg/robot"
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

// stored is a robot as the system stores it after a scan: first seen and
// last seen now.
func stored(id uuid.UUID, serial string, gatewayID uuid.UUID) domain.Robot {
	r := scanned(id, serial, gatewayID)
	r.FirstSeenAt = now
	r.LastSeenAt = ptr(now)
	return r
}

// scan reports robots and fails the test on error.
func scan(t *testing.T, s *robot.Service, robots ...domain.Robot) {
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
			repo := &robot.FakeRepository{}
			s := robot.NewService(repo)
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
			s := robot.NewService(&robot.FakeRepository{})
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
			s := robot.NewService(&robot.FakeRepository{})
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

// measurement is a measurement of robot1, as a gateway sends it.
func measurement(measuredAt time.Time) domain.Measurement {
	return domain.Measurement{
		RobotID:          robot1ID,
		BatteryPct:       ptr(80.0),
		Position:         &domain.Position{MapID: "floor-1", X: 1, Y: 2, HeadingRad: 0.5},
		Activity:         domain.ActivityMoving,
		ActiveFaultCodes: []string{},
		MeasuredAt:       measuredAt,
	}
}

// received is m as the system stores it: with the time it arrived.
func received(m domain.Measurement) domain.Measurement {
	m.ReceivedAt = now
	return m
}

// fault is a fault of robot1, as a gateway sends it.
func fault(code string, occurredAt time.Time) domain.Fault {
	return domain.Fault{
		RobotID:    robot1ID,
		Code:       code,
		Severity:   domain.SeverityCritical,
		Message:    "motor overheated",
		OccurredAt: occurredAt,
	}
}

// receivedFault is f as the system stores it: with the time it arrived.
func receivedFault(f domain.Fault) domain.Fault {
	f.ReceivedAt = now
	return f
}

func TestReportMeasurement(t *testing.T) {
	measuredAt := now.Add(-time.Second)

	tests := []struct {
		name        string
		gateway     uuid.UUID
		measurement domain.Measurement
		earlier     []domain.Measurement // reported before the one under test
		wantStored  []domain.Measurement
		wantErr     error
	}{
		{
			name:        "robot of this gateway",
			gateway:     gateway1ID,
			measurement: measurement(measuredAt),
			wantStored: []domain.Measurement{
				received(measurement(measuredAt)),
			},
		},
		{
			name:        "measurement sent twice is stored once",
			gateway:     gateway1ID,
			measurement: measurement(measuredAt),
			earlier: []domain.Measurement{
				measurement(measuredAt),
			},
			wantStored: []domain.Measurement{
				received(measurement(measuredAt)),
			},
		},
		{
			name:        "robot of another gateway",
			gateway:     gateway2ID,
			measurement: measurement(measuredAt),
			wantStored:  nil,
			wantErr:     domain.ErrOtherGateway,
		},
		{
			name:    "unknown robot",
			gateway: gateway1ID,
			measurement: domain.Measurement{
				RobotID:    uuid.New(),
				MeasuredAt: measuredAt,
			},
			wantStored: nil,
			wantErr:    domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &robot.FakeRepository{}
			s := robot.NewService(repo)
			scan(t, s, scanned(robot1ID, "SN-1", gateway1ID))
			for _, m := range tt.earlier {
				if err := s.ReportMeasurement(context.Background(), gateway1ID, m); err != nil {
					t.Fatalf("earlier measurement: %v", err)
				}
			}

			err := s.ReportMeasurement(context.Background(), tt.gateway, tt.measurement)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.wantStored, repo.Measurements, approxTime); diff != "" {
				t.Errorf("stored measurements mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReportFault(t *testing.T) {
	occurredAt := now.Add(-time.Second)

	tests := []struct {
		name       string
		gateway    uuid.UUID
		fault      domain.Fault
		earlier    []domain.Fault // reported before the one under test
		wantStored []domain.Fault
		wantErr    error
	}{
		{
			name:    "robot of this gateway",
			gateway: gateway1ID,
			fault:   fault("E_MOTOR", occurredAt),
			wantStored: []domain.Fault{
				receivedFault(fault("E_MOTOR", occurredAt)),
			},
		},
		{
			name:    "fault sent twice is stored once",
			gateway: gateway1ID,
			fault:   fault("E_MOTOR", occurredAt),
			earlier: []domain.Fault{
				fault("E_MOTOR", occurredAt),
			},
			wantStored: []domain.Fault{
				receivedFault(fault("E_MOTOR", occurredAt)),
			},
		},
		{
			name:    "same code at another time is another fault",
			gateway: gateway1ID,
			fault:   fault("E_MOTOR", occurredAt),
			earlier: []domain.Fault{
				fault("E_MOTOR", occurredAt.Add(-time.Hour)),
			},
			wantStored: []domain.Fault{
				receivedFault(fault("E_MOTOR", occurredAt.Add(-time.Hour))),
				receivedFault(fault("E_MOTOR", occurredAt)),
			},
		},
		{
			name:       "robot of another gateway",
			gateway:    gateway2ID,
			fault:      fault("E_MOTOR", occurredAt),
			wantStored: nil,
			wantErr:    domain.ErrOtherGateway,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &robot.FakeRepository{}
			s := robot.NewService(repo)
			scan(t, s, scanned(robot1ID, "SN-1", gateway1ID))
			for _, f := range tt.earlier {
				if err := s.ReportFault(context.Background(), gateway1ID, f); err != nil {
					t.Fatalf("earlier fault: %v", err)
				}
			}

			err := s.ReportFault(context.Background(), tt.gateway, tt.fault)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.wantStored, repo.Faults, approxTime); diff != "" {
				t.Errorf("stored faults mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListMeasurements(t *testing.T) {
	from := now.Add(-time.Hour)
	to := now

	tests := []struct {
		name    string
		robot   uuid.UUID
		want    []domain.Measurement
		wantErr error
	}{
		{
			name:  "only the range",
			robot: robot1ID,
			want: []domain.Measurement{
				received(measurement(from)),
				received(measurement(to.Add(-time.Minute))),
			},
		},
		{
			name:    "unknown robot",
			robot:   uuid.New(),
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := robot.NewService(&robot.FakeRepository{})
			scan(t, s, scanned(robot1ID, "SN-1", gateway1ID))
			// One before the range, two in it, one at its exclusive end.
			for _, at := range []time.Time{from.Add(-time.Minute), from, to.Add(-time.Minute), to} {
				if err := s.ReportMeasurement(context.Background(), gateway1ID, measurement(at)); err != nil {
					t.Fatalf("report measurement: %v", err)
				}
			}

			got, err := s.ListMeasurements(context.Background(), tt.robot, from, to)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("measurements mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListFaults(t *testing.T) {
	from := now.Add(-time.Hour)
	to := now

	tests := []struct {
		name    string
		robot   uuid.UUID
		want    []domain.Fault
		wantErr error
	}{
		{
			name:  "only the range",
			robot: robot1ID,
			want: []domain.Fault{
				receivedFault(fault("E_MOTOR", from)),
			},
		},
		{
			name:    "unknown robot",
			robot:   uuid.New(),
			wantErr: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := robot.NewService(&robot.FakeRepository{})
			scan(t, s, scanned(robot1ID, "SN-1", gateway1ID))
			for _, f := range []domain.Fault{fault("E_OLD", from.Add(-time.Minute)), fault("E_MOTOR", from)} {
				if err := s.ReportFault(context.Background(), gateway1ID, f); err != nil {
					t.Fatalf("report fault: %v", err)
				}
			}

			got, err := s.ListFaults(context.Background(), tt.robot, from, to)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got, approxTime); diff != "" {
				t.Errorf("faults mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
