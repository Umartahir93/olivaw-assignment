//go:build unit

package handler_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	"github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
	robothandler "github.com/umartahir93/olivaw-assignment/pkg/robot/handler"
)

var (
	firstSeenAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Connectivity is derived from the current time, so this is relative
	// to it.
	recently = time.Now().Add(-time.Minute).UTC()

	gatewayID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	// robot1 sent a measurement a minute ago.
	robot1 = domain.Robot{
		ID:           uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		SerialNumber: "SN-1",
		GatewayID:    gatewayID,
		FirstSeenAt:  firstSeenAt,
		LastSeenAt:   &recently,
	}
	// robot2 was scanned but never sent anything.
	robot2 = domain.Robot{
		ID:           uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		SerialNumber: "SN-2",
		GatewayID:    gatewayID,
		FirstSeenAt:  firstSeenAt,
	}
	unknownID = uuid.MustParse("99999999-9999-9999-9999-999999999999")

	errDatabase = errors.New("database down")

	robotNotFound = fleetv1.Problem{
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: fleetv1.NewOptString("robot not found"),
	}
)

// robots returns a robot service that knows robot1 and robot2.
func robots() *robothandler.FakeRobotService {
	return &robothandler.FakeRobotService{
		Robots: []domain.Robot{robot1, robot2},
	}
}

func TestGetRobot(t *testing.T) {
	tests := []struct {
		name    string
		id      uuid.UUID
		robots  *robothandler.FakeRobotService
		want    fleetv1.GetRobotRes
		wantErr error
	}{
		{
			name:   "online robot",
			id:     robot1.ID,
			robots: robots(),
			want: robotREST(fleetv1.Robot{
				RobotId:      robot1.ID,
				SerialNumber: "SN-1",
				GatewayId:    gatewayID,
				FirstSeenAt:  firstSeenAt,
				Connectivity: fleetv1.ConnectivityOnline,
				LastSeenAt:   fleetv1.NewOptNilDateTime(recently),
			}),
		},
		{
			name:   "robot that never sent anything",
			id:     robot2.ID,
			robots: robots(),
			want: robotREST(fleetv1.Robot{
				RobotId:      robot2.ID,
				SerialNumber: "SN-2",
				GatewayId:    gatewayID,
				FirstSeenAt:  firstSeenAt,
				Connectivity: fleetv1.ConnectivityUnknown,
				LastSeenAt:   null(),
			}),
		},
		{
			name: "robot with its newest measurement",
			id:   robot1.ID,
			robots: &robothandler.FakeRobotService{
				Robots: []domain.Robot{withLatest(robot1, fullMeasurement)},
			},
			want: &fleetv1.Robot{
				RobotId:      robot1.ID,
				SerialNumber: "SN-1",
				GatewayId:    gatewayID,
				FirstSeenAt:  firstSeenAt,
				Connectivity: fleetv1.ConnectivityOnline,
				LastSeenAt:   fleetv1.NewOptNilDateTime(recently),
				Latest: fleetv1.NewOptNilMeasurement(fleetv1.Measurement{
					BatteryPct:       fleetv1.NewOptNilFloat64(80),
					Position:         fleetv1.NewOptNilPosition(fleetv1.Position{MapId: "floor-1", X: 1, Y: 2, HeadingRad: 0.5}),
					Activity:         fleetv1.ActivityMoving,
					ActiveFaultCodes: []string{"E_LIDAR"},
					MeasuredAt:       rangeFrom,
				}),
			},
		},
		{
			name:   "unknown robot",
			id:     unknownID,
			robots: robots(),
			want:   ptr(fleetv1.GetRobotNotFound(robotNotFound)),
		},
		{
			name: "service fails",
			id:   robot1.ID,
			robots: &robothandler.FakeRobotService{
				Err: errDatabase,
			},
			wantErr: errDatabase,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := robothandler.NewHandler(tt.robots).GetRobot(context.Background(), fleetv1.GetRobotParams{RobotId: tt.id})

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				validate(t, got)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

var (
	rangeFrom = time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	rangeTo   = rangeFrom.Add(time.Hour)

	// A measurement with every value reported, and one with none of the
	// optional ones.
	fullMeasurement = domain.Measurement{
		RobotID:          robot1.ID,
		BatteryPct:       ptr(80.0),
		Position:         &domain.Position{MapID: "floor-1", X: 1, Y: 2, HeadingRad: 0.5},
		Activity:         domain.ActivityMoving,
		ActiveFaultCodes: []string{"E_LIDAR"},
		MeasuredAt:       rangeFrom,
	}
	bareMeasurement = domain.Measurement{
		RobotID:    robot1.ID,
		Activity:   domain.ActivityUnknown,
		MeasuredAt: rangeFrom.Add(time.Second),
	}
	robot1Fault = domain.Fault{
		RobotID:    robot1.ID,
		Code:       "E_MOTOR",
		Severity:   domain.SeverityCritical,
		Message:    "motor overheated",
		OccurredAt: rangeFrom,
	}

	badRange = fleetv1.Problem{
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: fleetv1.NewOptString("the time range must not be longer than 24 hours"),
	}
	reversedRange = fleetv1.Problem{
		Title:  "Bad Request",
		Status: http.StatusBadRequest,
		Detail: fleetv1.NewOptString("to must not be before from"),
	}
)

// timeRangeCase is a from / to pair and the range the service should get.
type timeRangeCase struct {
	name     string
	from, to fleetv1.OptDateTime
	wantFrom time.Time // zero: to minus 24 hours
	wantTo   time.Time // zero: now
}

// timeRangeCases are the ranges both lists accept.
var timeRangeCases = []timeRangeCase{
	{
		name:     "from and to",
		from:     fleetv1.NewOptDateTime(rangeFrom),
		to:       fleetv1.NewOptDateTime(rangeTo),
		wantFrom: rangeFrom,
		wantTo:   rangeTo,
	},
	{
		name:     "only to: the 24 hours before it",
		to:       fleetv1.NewOptDateTime(rangeTo),
		wantFrom: rangeTo.Add(-24 * time.Hour),
		wantTo:   rangeTo,
	},
	{
		name: "neither: the last 24 hours",
	},
}

// checkRange checks the range the service got against tc, with now for
// what tc leaves out.
func checkRange(t *testing.T, tc timeRangeCase, from, to time.Time) {
	t.Helper()
	wantTo := tc.wantTo
	if wantTo.IsZero() {
		wantTo = time.Now()
	}
	wantFrom := tc.wantFrom
	if wantFrom.IsZero() {
		wantFrom = wantTo.Add(-24 * time.Hour)
	}
	approx := cmpopts.EquateApproxTime(time.Second)
	if diff := cmp.Diff(wantFrom, from, approx); diff != "" {
		t.Errorf("from mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantTo, to, approx); diff != "" {
		t.Errorf("to mismatch (-want +got):\n%s", diff)
	}
}

func TestListRobotMeasurements(t *testing.T) {
	tests := []struct {
		name     string
		id       uuid.UUID
		from, to fleetv1.OptDateTime
		robots   *robothandler.FakeRobotService
		want     fleetv1.ListRobotMeasurementsRes
		wantErr  error
	}{
		{
			name: "stored measurements",
			id:   robot1.ID,
			robots: &robothandler.FakeRobotService{
				Robots:       []domain.Robot{robot1},
				Measurements: []domain.Measurement{fullMeasurement, bareMeasurement},
			},
			want: &fleetv1.MeasurementList{
				Items: []fleetv1.Measurement{
					{
						BatteryPct:       fleetv1.NewOptNilFloat64(80),
						Position:         fleetv1.NewOptNilPosition(fleetv1.Position{MapId: "floor-1", X: 1, Y: 2, HeadingRad: 0.5}),
						Activity:         fleetv1.ActivityMoving,
						ActiveFaultCodes: []string{"E_LIDAR"},
						MeasuredAt:       rangeFrom,
					},
					{
						BatteryPct:       nullFloat(),
						Position:         nullPosition(),
						Activity:         fleetv1.ActivityUnknown,
						ActiveFaultCodes: []string{},
						MeasuredAt:       rangeFrom.Add(time.Second),
					},
				},
			},
		},
		{
			name:   "no measurements",
			id:     robot2.ID,
			robots: robots(),
			want: &fleetv1.MeasurementList{
				Items: []fleetv1.Measurement{},
			},
		},
		{
			name:   "range longer than 24 hours",
			id:     robot1.ID,
			from:   fleetv1.NewOptDateTime(rangeFrom),
			to:     fleetv1.NewOptDateTime(rangeFrom.Add(25 * time.Hour)),
			robots: robots(),
			want:   ptr(fleetv1.ListRobotMeasurementsBadRequest(badRange)),
		},
		{
			name:   "to before from",
			id:     robot1.ID,
			from:   fleetv1.NewOptDateTime(rangeTo),
			to:     fleetv1.NewOptDateTime(rangeFrom),
			robots: robots(),
			want:   ptr(fleetv1.ListRobotMeasurementsBadRequest(reversedRange)),
		},
		{
			name:   "unknown robot",
			id:     unknownID,
			robots: robots(),
			want:   ptr(fleetv1.ListRobotMeasurementsNotFound(robotNotFound)),
		},
		{
			name: "service fails",
			id:   robot1.ID,
			robots: &robothandler.FakeRobotService{
				Err: errDatabase,
			},
			wantErr: errDatabase,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := fleetv1.ListRobotMeasurementsParams{RobotId: tt.id, From: tt.from, To: tt.to}

			got, err := robothandler.NewHandler(tt.robots).ListRobotMeasurements(context.Background(), params)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				validate(t, got)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListRobotFaults(t *testing.T) {
	tests := []struct {
		name     string
		id       uuid.UUID
		from, to fleetv1.OptDateTime
		robots   *robothandler.FakeRobotService
		want     fleetv1.ListRobotFaultsRes
		wantErr  error
	}{
		{
			name: "stored faults",
			id:   robot1.ID,
			robots: &robothandler.FakeRobotService{
				Robots: []domain.Robot{robot1},
				Faults: []domain.Fault{robot1Fault},
			},
			want: &fleetv1.FaultList{
				Items: []fleetv1.Fault{
					{
						Code:       "E_MOTOR",
						Severity:   fleetv1.FaultSeverityCritical,
						Message:    "motor overheated",
						OccurredAt: rangeFrom,
					},
				},
			},
		},
		{
			name:   "no faults",
			id:     robot2.ID,
			robots: robots(),
			want: &fleetv1.FaultList{
				Items: []fleetv1.Fault{},
			},
		},
		{
			name:   "range longer than 24 hours",
			id:     robot1.ID,
			from:   fleetv1.NewOptDateTime(rangeFrom),
			to:     fleetv1.NewOptDateTime(rangeFrom.Add(25 * time.Hour)),
			robots: robots(),
			want:   ptr(fleetv1.ListRobotFaultsBadRequest(badRange)),
		},
		{
			name:   "unknown robot",
			id:     unknownID,
			robots: robots(),
			want:   ptr(fleetv1.ListRobotFaultsNotFound(robotNotFound)),
		},
		{
			name: "service fails",
			id:   robot1.ID,
			robots: &robothandler.FakeRobotService{
				Err: errDatabase,
			},
			wantErr: errDatabase,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := fleetv1.ListRobotFaultsParams{RobotId: tt.id, From: tt.from, To: tt.to}

			got, err := robothandler.NewHandler(tt.robots).ListRobotFaults(context.Background(), params)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				validate(t, got)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTimeRange(t *testing.T) {
	for _, tc := range timeRangeCases {
		t.Run("measurements, "+tc.name, func(t *testing.T) {
			robots := robots()
			params := fleetv1.ListRobotMeasurementsParams{RobotId: robot1.ID, From: tc.from, To: tc.to}

			if _, err := robothandler.NewHandler(robots).ListRobotMeasurements(context.Background(), params); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			checkRange(t, tc, robots.From, robots.To)
		})
		t.Run("faults, "+tc.name, func(t *testing.T) {
			robots := robots()
			params := fleetv1.ListRobotFaultsParams{RobotId: robot1.ID, From: tc.from, To: tc.to}

			if _, err := robothandler.NewHandler(robots).ListRobotFaults(context.Background(), params); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			checkRange(t, tc, robots.From, robots.To)
		})
	}
}

// nullFloat returns a null number.
func nullFloat() fleetv1.OptNilFloat64 {
	var f fleetv1.OptNilFloat64
	f.SetToNull()
	return f
}

// nullPosition returns a null position.
func nullPosition() fleetv1.OptNilPosition {
	var p fleetv1.OptNilPosition
	p.SetToNull()
	return p
}

func ptr[T any](v T) *T {
	return &v
}

// null returns a null timestamp.
func null() fleetv1.OptNilDateTime {
	var t fleetv1.OptNilDateTime
	t.SetToNull()
	return t
}

// withLatest returns r with m as its newest measurement.
func withLatest(r domain.Robot, m domain.Measurement) domain.Robot {
	r.Latest = &m
	return r
}

// robotREST completes r with a null latest measurement.
func robotREST(r fleetv1.Robot) *fleetv1.Robot {
	r.Latest.SetToNull()
	return &r
}

// validate checks a response against the API contract, using the
// validators ogen generates from apispecs/rest/openapi.yaml. Problem
// responses have no validator.
func validate(t *testing.T, res any) {
	t.Helper()
	v, ok := res.(interface{ Validate() error })
	if !ok {
		return
	}
	if err := v.Validate(); err != nil {
		t.Errorf("response breaks the API contract: %v", err)
	}
}
