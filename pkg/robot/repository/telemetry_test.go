//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

var (
	measuredAt = firstSeenAt.Add(10 * time.Minute)

	// fullMeasurement has every value reported; bareMeasurement none of the
	// optional ones.
	fullMeasurement = domain.Measurement{
		RobotID:          robot1.ID,
		BatteryPct:       ptr(80.5),
		Position:         &domain.Position{MapID: "floor-1", X: 12.5, Y: 3, HeadingRad: 1.57},
		Activity:         domain.ActivityMoving,
		ActiveFaultCodes: []string{"E_LIDAR", "E_MOTOR"},
		MeasuredAt:       measuredAt,
		ReceivedAt:       measuredAt.Add(time.Second),
	}
	bareMeasurement = domain.Measurement{
		RobotID:          robot1.ID,
		Activity:         domain.ActivityUnknown,
		ActiveFaultCodes: []string{},
		MeasuredAt:       measuredAt.Add(time.Second),
		ReceivedAt:       measuredAt.Add(2 * time.Second),
	}

	// sameTimeAsFull is another measurement of robot1 taken at the same
	// moment as fullMeasurement.
	sameTimeAsFull = domain.Measurement{
		RobotID:          robot1.ID,
		Activity:         domain.ActivityIdle,
		ActiveFaultCodes: []string{},
		MeasuredAt:       measuredAt,
		ReceivedAt:       measuredAt.Add(time.Minute),
	}

	motorFault = domain.Fault{
		RobotID:    robot1.ID,
		Code:       "E_MOTOR",
		Severity:   domain.SeverityCritical,
		Message:    "motor overheated",
		OccurredAt: measuredAt,
		ReceivedAt: measuredAt.Add(time.Second),
	}
)

func ptr[T any](v T) *T { return &v }

// at returns m taken at measuredAt and received a second later.
func at(m domain.Measurement, measuredAt time.Time) domain.Measurement {
	m.MeasuredAt = measuredAt
	m.ReceivedAt = measuredAt.Add(time.Second)
	return m
}

func TestCreateMeasurement(t *testing.T) {
	tests := []struct {
		name       string
		earlier    []domain.Measurement // stored before the one under test
		create     domain.Measurement
		wantStored []domain.Measurement
		wantErr    bool
	}{
		{
			name:   "every value reported",
			create: fullMeasurement,
			wantStored: []domain.Measurement{
				fullMeasurement,
			},
		},
		{
			name:   "no optional value reported",
			create: bareMeasurement,
			wantStored: []domain.Measurement{
				bareMeasurement,
			},
		},
		{
			name: "measurement stored before is not stored again",
			earlier: []domain.Measurement{
				fullMeasurement,
			},
			create: sameTimeAsFull,
			wantStored: []domain.Measurement{
				fullMeasurement,
			},
		},
		{
			name:       "robot that is not stored",
			create:     at(domain.Measurement{RobotID: robotOfUnknownGateway.ID, Activity: domain.ActivityIdle}, measuredAt),
			wantStored: []domain.Measurement{},
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newRepository(t, robot1)
			for _, m := range tt.earlier {
				if _, err := repo.CreateMeasurement(ctx, m); err != nil {
					t.Fatalf("earlier measurement: %v", err)
				}
			}

			_, err := repo.CreateMeasurement(ctx, tt.create)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want an error: %t", err, tt.wantErr)
			}
			stored, err := repo.ListMeasurements(ctx, robot1.ID, firstSeenAt, firstSeenAt.Add(time.Hour))
			if err != nil {
				t.Fatalf("list measurements: %v", err)
			}
			if diff := cmp.Diff(tt.wantStored, stored, equateEmpty); diff != "" {
				t.Errorf("stored measurements mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListMeasurements(t *testing.T) {
	from := measuredAt
	to := measuredAt.Add(time.Hour)
	robot2Measurement := fullMeasurement
	robot2Measurement.RobotID = robot2.ID

	tests := []struct {
		name  string
		robot domain.Robot
		want  []domain.Measurement
	}{
		{
			name:  "the robot's measurements in the range, oldest first",
			robot: robot1,
			want: []domain.Measurement{
				at(fullMeasurement, from),
				at(fullMeasurement, from.Add(time.Minute)),
			},
		},
		{
			name:  "robot without measurements",
			robot: robot9,
			want:  []domain.Measurement{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newRepository(t, robot1, robot2, robot9)
			// Stored newest first, to check the order. The range includes from
			// and excludes to.
			for _, m := range []domain.Measurement{
				at(fullMeasurement, to),
				at(fullMeasurement, from.Add(time.Minute)),
				at(robot2Measurement, from.Add(time.Minute)),
				at(fullMeasurement, from),
				at(fullMeasurement, from.Add(-time.Minute)),
			} {
				if _, err := repo.CreateMeasurement(ctx, m); err != nil {
					t.Fatalf("create measurement: %v", err)
				}
			}

			got, err := repo.ListMeasurements(ctx, tt.robot.ID, from, to)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, equateEmpty); diff != "" {
				t.Errorf("measurements mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateAndListFaults(t *testing.T) {
	sameCodeLater := motorFault
	sameCodeLater.OccurredAt = motorFault.OccurredAt.Add(time.Minute)
	resent := motorFault
	resent.Message = "sent again"
	outOfRange := motorFault
	outOfRange.OccurredAt = motorFault.OccurredAt.Add(-time.Hour)

	ctx := context.Background()
	repo := newRepository(t, robot1)
	for _, f := range []domain.Fault{sameCodeLater, motorFault, resent, outOfRange} {
		if _, err := repo.CreateFault(ctx, f); err != nil {
			t.Fatalf("create fault: %v", err)
		}
	}

	got, err := repo.ListFaults(ctx, robot1.ID, motorFault.OccurredAt, motorFault.OccurredAt.Add(time.Hour))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The resent fault is not stored again; the same code later is another
	// fault.
	want := []domain.Fault{motorFault, sameCodeLater}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("faults mismatch (-want +got):\n%s", diff)
	}
}

// TestRobotLastSeen: a robot is read with its newest measurement, which also
// tells when it was last seen.
func TestRobotLastSeen(t *testing.T) {
	scannedAt := measuredAt

	tests := []struct {
		name         string
		measurements []domain.Measurement
		want         *time.Time
		wantLatest   *domain.Measurement
	}{
		{
			name:       "no measurements: when the scan saw it",
			want:       &scannedAt,
			wantLatest: nil,
		},
		{
			name: "measurements: when the newest one arrived",
			measurements: []domain.Measurement{
				at(fullMeasurement, scannedAt.Add(time.Minute)),
				at(bareMeasurement, scannedAt.Add(2*time.Minute)),
			},
			want:       ptr(scannedAt.Add(2*time.Minute + time.Second)),
			wantLatest: ptr(at(bareMeasurement, scannedAt.Add(2*time.Minute))),
		},
		{
			name: "measurement older than the scan: when the scan saw it",
			measurements: []domain.Measurement{
				at(fullMeasurement, scannedAt.Add(-time.Hour)),
			},
			want:       &scannedAt,
			wantLatest: ptr(at(fullMeasurement, scannedAt.Add(-time.Hour))),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newRepository(t, withLastSeen(robot1, scannedAt))
			for _, m := range tt.measurements {
				if _, err := repo.CreateMeasurement(ctx, m); err != nil {
					t.Fatalf("create measurement: %v", err)
				}
			}

			got, err := repo.GetRobot(ctx, robot1.ID)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got.LastSeenAt); diff != "" {
				t.Errorf("last seen mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantLatest, got.Latest, equateEmpty); diff != "" {
				t.Errorf("latest measurement mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
