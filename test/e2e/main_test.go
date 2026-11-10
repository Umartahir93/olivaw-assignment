//go:build e2e

// Package e2e tests the running system over its APIs, scenario by scenario
// in Given / When / Then steps: REST as the operator and the fleet manager,
// gRPC as the gateway. Start the system first, e.g. with `make up`.
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
)

// Where the system runs; the defaults match docker-compose.yml.
var (
	restURL  = envOr("FLEET_REST_URL", "http://localhost:8080")
	grpcAddr = envOr("FLEET_GRPC_ADDR", "localhost:9090")
)

// The clients every scenario uses. The REST client is generated from
// openapi.yaml and validates every response against it.
var (
	rest           *fleetv1.Client
	gatewayService gatewayv1.GatewayServiceClient
	robotService   gatewayv1.RobotServiceClient
)

func TestMain(m *testing.M) {
	if err := checkHealth(); err != nil {
		fmt.Fprintf(os.Stderr, "system not ready at %s (start it with `make up`): %v\n", restURL, err)
		os.Exit(1)
	}

	var err error
	rest, err = fleetv1.NewClient(restURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rest client:", err)
		os.Exit(1)
	}
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "grpc client:", err)
		os.Exit(1)
	}
	gatewayService = gatewayv1.NewGatewayServiceClient(conn)
	robotService = gatewayv1.NewRobotServiceClient(conn)

	code := m.Run()
	_ = conn.Close()
	os.Exit(code)
}

// checkHealth asks the server's health endpoint whether it is ready.
func checkHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restURL+"/healthz", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: %s", res.Status)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
