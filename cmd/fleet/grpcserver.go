package main

import (
	"github.com/umartahir93/olivaw-assignment/pkg/telemetry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	telemetryv1 "github.com/umartahir93/olivaw-assignment/gen/telemetry/v1"
)

// newGRPCServer returns a gRPC server with the TelemetryService registered.
func newGRPCServer() *grpc.Server {
	srv := grpc.NewServer()
	telemetryv1.RegisterTelemetryServiceServer(srv, &telemetry.Server{})
	reflection.Register(srv)
	return srv
}
