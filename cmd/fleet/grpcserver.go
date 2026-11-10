package main

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	"github.com/umartahir93/olivaw-assignment/pkg/telemetry/adapter"
)

// newGRPCServer returns a gRPC server with the gateway-facing services
// registered.
func newGRPCServer() *grpc.Server {
	srv := grpc.NewServer()
	gatewayv1.RegisterGatewayServiceServer(srv, &adapter.GatewayServer{})
	gatewayv1.RegisterRobotServiceServer(srv, &adapter.RobotServer{})
	reflection.Register(srv)
	return srv
}
