// Package main runs the fleet server: the gRPC API for gateways and the REST
// API for people.
package main

import (
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	"github.com/umartahir93/olivaw-assignment/middleware"
	"github.com/umartahir93/olivaw-assignment/pkg/gateway"
	gatewaygrpc "github.com/umartahir93/olivaw-assignment/pkg/gateway/grpc"
	"github.com/umartahir93/olivaw-assignment/pkg/robot"
	robotgrpc "github.com/umartahir93/olivaw-assignment/pkg/robot/grpc"
)

// newGRPCServer returns a gRPC server with the gateway-facing services and
// the standard health service (grpc.health.v1) registered. Every call is
// logged, and a panic becomes INTERNAL instead of stopping the server.
func newGRPCServer(log *slog.Logger, gateways *gateway.Service, robots *robot.Service) (*grpc.Server, *health.Server) {
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		middleware.UnaryLogging(log),
		middleware.UnaryRecovery(),
	))
	gatewayv1.RegisterGatewayServiceServer(srv, gatewaygrpc.NewServer(gateways, robots))
	gatewayv1.RegisterRobotServiceServer(srv, robotgrpc.NewServer(gateways, robots))

	// SERVING for the whole server (""); Shutdown flips it to NOT_SERVING.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)

	reflection.Register(srv)
	return srv, healthSrv
}
