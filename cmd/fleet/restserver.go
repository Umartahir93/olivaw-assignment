package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
	"github.com/umartahir93/olivaw-assignment/middleware"
	"github.com/umartahir93/olivaw-assignment/pkg/gateway"
	gatewayhandler "github.com/umartahir93/olivaw-assignment/pkg/gateway/handler"
	"github.com/umartahir93/olivaw-assignment/pkg/robot"
	robothandler "github.com/umartahir93/olivaw-assignment/pkg/robot/handler"
)

// healthPath is where the server answers 200 while it can reach the
// database, for container health checks. It is not part of the API contract.
const healthPath = "/healthz"

// newRESTServer returns an HTTP server for the fleet REST API on addr. The
// generated server routes requests and validates them against
// apispecs/rest/openapi.yaml before calling the gateway and robot handlers.
// Every request is logged, and a panic becomes a 500.
func newRESTServer(addr string, log *slog.Logger, db *sqlx.DB, gateways *gateway.Service, robots *robot.Service) (*http.Server, error) {
	h := &restHandler{
		gatewayHandler: gatewayhandler.NewHandler(gateways, robots),
		robotHandler:   robothandler.NewHandler(robots),
		log:            log,
	}
	api, err := fleetv1.NewServer(h, fleetv1.WithErrorHandler(problemErrorHandler))
	if err != nil {
		return nil, fmt.Errorf("rest server: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle(healthPath, healthHandler(db))
	mux.Handle("/", api)
	return &http.Server{
		Addr:              addr,
		Handler:           middleware.HTTPLogging(log, middleware.HTTPRecovery(log, mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

// healthHandler answers 200 if the database answers, 503 otherwise.
func healthHandler(db *sqlx.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

// The embedded handlers are both named Handler; the aliases give the fields
// distinct names.
type (
	gatewayHandler = gatewayhandler.Handler
	robotHandler   = robothandler.Handler
)

// restHandler is the one fleetv1.Handler the generated server needs: the
// gateway and robot handlers together.
type restHandler struct {
	*gatewayHandler
	*robotHandler

	log *slog.Logger
}

var _ fleetv1.Handler = (*restHandler)(nil)

// NewError logs an error returned by a handler and turns it into a 500
// problem without details, so internals never leak.
func (h *restHandler) NewError(ctx context.Context, err error) *fleetv1.ErrorStatusCode {
	h.log.ErrorContext(ctx, "rest handler failed", "error", err)
	return &fleetv1.ErrorStatusCode{
		StatusCode: http.StatusInternalServerError,
		Response:   gatewayhandler.Problem(http.StatusInternalServerError, ""),
	}
}
