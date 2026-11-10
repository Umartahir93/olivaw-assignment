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
	gatewaydomain "github.com/umartahir93/olivaw-assignment/pkg/gateway/domain"
	gatewayhandler "github.com/umartahir93/olivaw-assignment/pkg/gateway/handler"
	robotdomain "github.com/umartahir93/olivaw-assignment/pkg/robot/domain"
)

var (
	registeredAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Connectivity is derived from the current time, so these are relative
	// to it.
	recently = time.Now().Add(-time.Minute).UTC()
	longAgo  = time.Now().Add(-time.Hour).UTC()

	// gateway1 has never called.
	gateway1 = gatewaydomain.Gateway{
		ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		SerialNumber: "GW-1",
		Location:     "Munich",
		Name:         ptr("Main hall"),
		RegisteredAt: registeredAt,
	}
	// gateway2 called a minute ago.
	gateway2 = gatewaydomain.Gateway{
		ID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		SerialNumber: "GW-2",
		Location:     "Berlin",
		RegisteredAt: registeredAt,
		LastSeenAt:   &recently,
	}
	robot1 = robotdomain.Robot{
		ID:           uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		SerialNumber: "SN-1",
		GatewayID:    gateway1.ID,
		FirstSeenAt:  registeredAt,
		LastSeenAt:   &longAgo,
	}

	errDatabase = errors.New("database down")
)

func ptr[T any](v T) *T {
	return &v
}

// null returns a null timestamp.
func null() fleetv1.OptNilDateTime {
	var t fleetv1.OptNilDateTime
	t.SetToNull()
	return t
}

// The REST representations of the fixtures.
var (
	gateway1REST = fleetv1.Gateway{
		GatewayId:    gateway1.ID,
		SerialNumber: "GW-1",
		Location:     "Munich",
		Name:         fleetv1.NewOptString("Main hall"),
		RegisteredAt: registeredAt,
		Connectivity: fleetv1.ConnectivityUnknown,
		LastSeenAt:   null(),
	}
	gateway2REST = fleetv1.Gateway{
		GatewayId:    gateway2.ID,
		SerialNumber: "GW-2",
		Location:     "Berlin",
		RegisteredAt: registeredAt,
		Connectivity: fleetv1.ConnectivityOnline,
		LastSeenAt:   fleetv1.NewOptNilDateTime(recently),
	}
	robot1REST = func() fleetv1.Robot {
		r := fleetv1.Robot{
			RobotId:      robot1.ID,
			SerialNumber: "SN-1",
			GatewayId:    gateway1.ID,
			FirstSeenAt:  registeredAt,
			Connectivity: fleetv1.ConnectivityOffline,
			LastSeenAt:   fleetv1.NewOptNilDateTime(longAgo),
		}
		r.Latest.SetToNull()
		return r
	}()
)

func TestCreateGateway(t *testing.T) {
	tests := []struct {
		name       string
		request    *fleetv1.GatewayCreate
		gateways   *gatewayhandler.FakeGatewayService
		want       fleetv1.CreateGatewayRes
		wantErr    error
		wantStored []gatewaydomain.Gateway
	}{
		{
			name: "gateway with a name",
			request: &fleetv1.GatewayCreate{
				SerialNumber: "GW-3",
				Location:     "Hamburg",
				Name:         fleetv1.NewOptString("Dock"),
			},
			gateways: &gatewayhandler.FakeGatewayService{},
			want: &fleetv1.GatewayHeaders{
				Response: fleetv1.Gateway{
					SerialNumber: "GW-3",
					Location:     "Hamburg",
					Name:         fleetv1.NewOptString("Dock"),
					Connectivity: fleetv1.ConnectivityUnknown,
					LastSeenAt:   null(),
				},
			},
			wantStored: []gatewaydomain.Gateway{
				{
					SerialNumber: "GW-3",
					Location:     "Hamburg",
					Name:         ptr("Dock"),
				},
			},
		},
		{
			name: "gateway without a name",
			request: &fleetv1.GatewayCreate{
				SerialNumber: "GW-3",
				Location:     "Hamburg",
			},
			gateways: &gatewayhandler.FakeGatewayService{},
			want: &fleetv1.GatewayHeaders{
				Response: fleetv1.Gateway{
					SerialNumber: "GW-3",
					Location:     "Hamburg",
					Connectivity: fleetv1.ConnectivityUnknown,
					LastSeenAt:   null(),
				},
			},
			wantStored: []gatewaydomain.Gateway{
				{
					SerialNumber: "GW-3",
					Location:     "Hamburg",
				},
			},
		},
		{
			name: "serial number taken",
			request: &fleetv1.GatewayCreate{
				SerialNumber: "GW-1",
				Location:     "Hamburg",
			},
			gateways: &gatewayhandler.FakeGatewayService{
				Err: gatewaydomain.ErrAlreadyExists,
			},
			want: &fleetv1.CreateGatewayConflict{
				Title:  "Conflict",
				Status: http.StatusConflict,
				Detail: fleetv1.NewOptString("a gateway with this serial number is already registered"),
			},
		},
		{
			name: "service fails",
			request: &fleetv1.GatewayCreate{
				SerialNumber: "GW-3",
				Location:     "Hamburg",
			},
			gateways: &gatewayhandler.FakeGatewayService{
				Err: errDatabase,
			},
			wantErr: errDatabase,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := gatewayhandler.NewHandler(tt.gateways, &gatewayhandler.FakeRobotService{})

			got, err := h.CreateGateway(context.Background(), tt.request)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				validate(t, got)
			}
			// The handler creates the gatewayId, so it is checked separately.
			ignoreID := cmpopts.IgnoreFields(fleetv1.Gateway{}, "GatewayId")
			ignoreLocation := cmpopts.IgnoreFields(fleetv1.GatewayHeaders{}, "Location")
			if diff := cmp.Diff(tt.want, got, ignoreID, ignoreLocation); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantStored, tt.gateways.Gateways, cmpopts.IgnoreFields(gatewaydomain.Gateway{}, "ID")); diff != "" {
				t.Errorf("stored gateways mismatch (-want +got):\n%s", diff)
			}
			if created, ok := got.(*fleetv1.GatewayHeaders); ok {
				id := tt.gateways.Gateways[0].ID
				if created.Response.GatewayId != id {
					t.Errorf("gatewayId = %s, want the stored %s", created.Response.GatewayId, id)
				}
				if want := "/v1/gateways/" + id.String(); created.Location.Value.Path != want {
					t.Errorf("Location = %q, want %q", created.Location.Value.Path, want)
				}
			}
		})
	}
}

func TestListGateways(t *testing.T) {
	tests := []struct {
		name     string
		gateways *gatewayhandler.FakeGatewayService
		want     *fleetv1.GatewayList
		wantErr  error
	}{
		{
			name: "all gateways",
			gateways: &gatewayhandler.FakeGatewayService{
				Gateways: []gatewaydomain.Gateway{gateway1, gateway2},
			},
			want: &fleetv1.GatewayList{
				Items: []fleetv1.Gateway{gateway1REST, gateway2REST},
			},
		},
		{
			name:     "no gateways",
			gateways: &gatewayhandler.FakeGatewayService{},
			want: &fleetv1.GatewayList{
				Items: []fleetv1.Gateway{},
			},
		},
		{
			name: "service fails",
			gateways: &gatewayhandler.FakeGatewayService{
				Err: errDatabase,
			},
			wantErr: errDatabase,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := gatewayhandler.NewHandler(tt.gateways, &gatewayhandler.FakeRobotService{})

			got, err := h.ListGateways(context.Background())

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

func TestGetGateway(t *testing.T) {
	tests := []struct {
		name     string
		id       uuid.UUID
		gateways *gatewayhandler.FakeGatewayService
		want     fleetv1.GetGatewayRes
		wantErr  error
	}{
		{
			name: "known gateway",
			id:   gateway1.ID,
			gateways: &gatewayhandler.FakeGatewayService{
				Gateways: []gatewaydomain.Gateway{gateway1, gateway2},
			},
			want: &gateway1REST,
		},
		{
			name: "unknown gateway",
			id:   uuid.MustParse("99999999-9999-9999-9999-999999999999"),
			gateways: &gatewayhandler.FakeGatewayService{
				Gateways: []gatewaydomain.Gateway{gateway1},
			},
			want: &fleetv1.GetGatewayNotFound{
				Title:  "Not Found",
				Status: http.StatusNotFound,
				Detail: fleetv1.NewOptString("gateway not found"),
			},
		},
		{
			name: "service fails",
			id:   gateway1.ID,
			gateways: &gatewayhandler.FakeGatewayService{
				Err: errDatabase,
			},
			wantErr: errDatabase,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := gatewayhandler.NewHandler(tt.gateways, &gatewayhandler.FakeRobotService{})

			got, err := h.GetGateway(context.Background(), fleetv1.GetGatewayParams{GatewayId: tt.id})

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

func TestListGatewayRobots(t *testing.T) {
	tests := []struct {
		name     string
		id       uuid.UUID
		gateways *gatewayhandler.FakeGatewayService
		robots   *gatewayhandler.FakeRobotService
		want     fleetv1.ListGatewayRobotsRes
		wantErr  error
	}{
		{
			name: "the gateway's robots",
			id:   gateway1.ID,
			gateways: &gatewayhandler.FakeGatewayService{
				Gateways: []gatewaydomain.Gateway{gateway1},
			},
			robots: &gatewayhandler.FakeRobotService{
				Robots: []robotdomain.Robot{robot1},
			},
			want: &fleetv1.RobotList{
				Items: []fleetv1.Robot{robot1REST},
			},
		},
		{
			name: "no robots",
			id:   gateway1.ID,
			gateways: &gatewayhandler.FakeGatewayService{
				Gateways: []gatewaydomain.Gateway{gateway1},
			},
			robots: &gatewayhandler.FakeRobotService{},
			want: &fleetv1.RobotList{
				Items: []fleetv1.Robot{},
			},
		},
		{
			name:     "unknown gateway",
			id:       gateway1.ID,
			gateways: &gatewayhandler.FakeGatewayService{},
			robots:   &gatewayhandler.FakeRobotService{},
			want: &fleetv1.ListGatewayRobotsNotFound{
				Title:  "Not Found",
				Status: http.StatusNotFound,
				Detail: fleetv1.NewOptString("gateway not found"),
			},
		},
		{
			name: "gateway service fails",
			id:   gateway1.ID,
			gateways: &gatewayhandler.FakeGatewayService{
				Err: errDatabase,
			},
			robots:  &gatewayhandler.FakeRobotService{},
			wantErr: errDatabase,
		},
		{
			name: "robot service fails",
			id:   gateway1.ID,
			gateways: &gatewayhandler.FakeGatewayService{
				Gateways: []gatewaydomain.Gateway{gateway1},
			},
			robots: &gatewayhandler.FakeRobotService{
				Err: errDatabase,
			},
			wantErr: errDatabase,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := gatewayhandler.NewHandler(tt.gateways, tt.robots)

			got, err := h.ListGatewayRobots(context.Background(), fleetv1.ListGatewayRobotsParams{GatewayId: tt.id})

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
