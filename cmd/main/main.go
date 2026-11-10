package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

var (
	host     = flag.String("host", "0.0.0.0", "gRPC server hostname")
	port     = flag.Int("port", 9090, "gRPC server port")
	logLevel slog.Level
)

func init() {
	// TextVar parses the level and rejects unknown names during flag.Parse.
	flag.TextVar(&logLevel, "log-level", slog.LevelInfo, "log level: debug, info, warn or error")
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	srv := newGRPCServer()

	// Graceful shutdown: on Ctrl-C or SIGTERM, stop accepting new requests
	// and let running ones finish. Serve then returns.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Info("shutting down")
		srv.GracefulStop()
	}()

	log.Info("grpc server listening", "addr", addr)
	if err := srv.Serve(lis); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
