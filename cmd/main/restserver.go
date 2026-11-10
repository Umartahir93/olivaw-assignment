package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/umartahir93/olivaw-assignment/api"
	fleetv1 "github.com/umartahir93/olivaw-assignment/gen/fleet/v1"
)

// newRESTServer returns an HTTP server for the fleet REST API on addr. The
// generated server routes requests and validates them against
// apispecs/rest/openapi.yaml before calling the handlers in api.
func newRESTServer(addr string) (*http.Server, error) {
	handler, err := fleetv1.NewServer(api.NewHandler(), fleetv1.WithErrorHandler(problemErrorHandler))
	if err != nil {
		return nil, fmt.Errorf("rest server: %w", err)
	}
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}, nil
}
