//go:build unit

package handler_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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

func TestListRobotMeasurements(t *testing.T) {
	tests := []struct {
		name    string
		id      uuid.UUID
		robots  *robothandler.FakeRobotService
		want    fleetv1.ListRobotMeasurementsRes
		wantErr error
	}{
		{
			name:   "known robot",
			id:     robot1.ID,
			robots: robots(),
			want: &fleetv1.MeasurementList{
				Items: []fleetv1.Measurement{},
			},
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
			params := fleetv1.ListRobotMeasurementsParams{RobotId: tt.id}

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
		name    string
		id      uuid.UUID
		robots  *robothandler.FakeRobotService
		want    fleetv1.ListRobotFaultsRes
		wantErr error
	}{
		{
			name:   "known robot",
			id:     robot1.ID,
			robots: robots(),
			want: &fleetv1.FaultList{
				Items: []fleetv1.Fault{},
			},
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
			params := fleetv1.ListRobotFaultsParams{RobotId: tt.id}

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

func ptr[T any](v T) *T {
	return &v
}

// null returns a null timestamp.
func null() fleetv1.OptNilDateTime {
	var t fleetv1.OptNilDateTime
	t.SetToNull()
	return t
}

// robotREST completes r with a null latest measurement; none are stored yet.
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
