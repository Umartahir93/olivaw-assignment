//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
)

// TestGatewayRegistration: an operator registers a gateway over REST before
// installing it.
func TestGatewayRegistration(t *testing.T) {
	t.Run("Register a gateway without a name", func(t *testing.T) {
		sc := newScenario()
		var gw gateway

		step(t, "When I register gateway GW-1 at Munich HQ without a name", func(t *testing.T) {
			gw = registerGateway(t, sc.serial("GW-1"))
		})
		step(t, "Then GW-1 has no name", func(t *testing.T) {
			if g := getGateway(t, gw.id); g.Name.Set {
				t.Errorf("name = %q, want none", g.Name.Value)
			}
		})
		step(t, "And GW-1 is in the list of gateways", func(t *testing.T) {
			list, err := rest.ListGateways(t.Context())
			if err != nil {
				t.Fatalf("list gateways: %v", err)
			}
			for _, g := range list.Items {
				if g.GatewayId == gw.id {
					return
				}
			}
			t.Errorf("gateway %s not in the list", gw.id)
		})
	})

	t.Run("A serial number is registered only once", func(t *testing.T) {
		sc := newScenario()
		var res fleetv1.CreateGatewayRes

		step(t, "Given gateway GW-1 is registered", func(t *testing.T) {
			registerGateway(t, sc.serial("GW-1"))
		})
		step(t, "When I register GW-1 again", func(t *testing.T) {
			res = register(t, sc.serial("GW-1"), "Berlin", nil)
		})
		step(t, "Then the registration is rejected with 409", func(t *testing.T) {
			conflict, ok := res.(*fleetv1.CreateGatewayConflict)
			if !ok {
				t.Fatalf("got %T, want a conflict", res)
			}
			if conflict.Status != http.StatusConflict {
				t.Errorf("status = %d, want %d", conflict.Status, http.StatusConflict)
			}
		})
	})

	t.Run("A registration that breaks the contract is rejected", func(t *testing.T) {
		tests := []struct {
			name       string
			body       map[string]string
			wantStatus int
		}{
			{
				name: "without a location",
				body: map[string]string{
					"serialNumber": "GW-1-" + uuid.NewString()[:8],
				},
				wantStatus: http.StatusBadRequest,
			},
			{
				name: "with an empty serial number",
				body: map[string]string{
					"serialNumber": "",
					"location":     "Munich HQ",
				},
				wantStatus: http.StatusBadRequest,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var status int

				step(t, "When I register a gateway "+tt.name, func(t *testing.T) {
					status = postRaw(t, "/v1/gateways", tt.body)
				})
				step(t, "Then the registration is rejected with 400", func(t *testing.T) {
					if status != tt.wantStatus {
						t.Errorf("status = %d, want %d", status, tt.wantStatus)
					}
				})
			})
		}
	})

	t.Run("An unknown gateway is not found", func(t *testing.T) {
		var res fleetv1.GetGatewayRes

		step(t, "When I look up a gateway that was never registered", func(t *testing.T) {
			var err error
			res, err = rest.GetGateway(t.Context(), fleetv1.GetGatewayParams{GatewayId: uuid.New()})
			if err != nil {
				t.Fatalf("get gateway: %v", err)
			}
		})
		step(t, "Then the answer is 404", func(t *testing.T) {
			if _, ok := res.(*fleetv1.GetGatewayNotFound); !ok {
				t.Errorf("got %T, want not found", res)
			}
		})
	})
}
