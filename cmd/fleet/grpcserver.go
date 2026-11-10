// Package main runs the fleet server: the gRPC API for gateways and the REST
// API for people.
package main

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	"github.com/umartahir93/olivaw-assignment/pkg/gateway"
	gatewaygrpc "github.com/umartahir93/olivaw-assignment/pkg/gateway/grpc"
	"github.com/umartahir93/olivaw-assignment/pkg/robot"
	robotgrpc "github.com/umartahir93/olivaw-assignment/pkg/robot/grpc"
)

// newGRPCServer returns a gRPC server with the gateway-facing services
// registered.
func newGRPCServer(gateways *gateway.Service, robots *robot.Service) *grpc.Server {
	srv := grpc.NewServer()
	gatewayv1.RegisterGatewayServiceServer(srv, gatewaygrpc.NewServer(gateways, robots))
	gatewayv1.RegisterRobotServiceServer(srv, robotgrpc.NewServer(gateways, robots))
	reflection.Register(srv)
	return srv
}
