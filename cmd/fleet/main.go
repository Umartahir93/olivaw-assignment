package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var (
	host     = flag.String("host", "0.0.0.0", "hostname both servers listen on")
	port     = flag.Int("port", 9090, "gRPC server port")
	restPort = flag.Int("rest-port", 8080, "REST server port")
	logLevel slog.Level
)

func init() {
	// TextVar parses the level and rejects unknown names during flag.Parse.
	flag.TextVar(&logLevel, "log-level", slog.LevelInfo, "log level: debug, info, warn or error")
}

// How long running requests may take to finish after a shutdown signal.
const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	grpcAddr := net.JoinHostPort(*host, strconv.Itoa(*port))
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", grpcAddr, err)
	}
	grpcSrv := newGRPCServer()

	restAddr := net.JoinHostPort(*host, strconv.Itoa(*restPort))
	restLis, err := net.Listen("tcp", restAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", restAddr, err)
	}
	restSrv, err := newRESTServer(restAddr)
	if err != nil {
		return err
	}

	// Run both servers until one fails or a shutdown signal arrives.
	serveErr := make(chan error, 2)
	go func() {
		log.Info("grpc server listening", "addr", grpcAddr)
		serveErr <- grpcSrv.Serve(grpcLis)
	}()
	go func() {
		log.Info("rest server listening", "addr", restAddr)
		if restErr := restSrv.Serve(restLis); !errors.Is(restErr, http.ErrServerClosed) {
			serveErr <- restErr
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err = <-serveErr:
		err = fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	// Graceful shutdown: stop accepting new requests and let running ones
	// finish.
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	grpcSrv.GracefulStop()
	if shutdownErr := restSrv.Shutdown(shutdownCtx); shutdownErr != nil {
		err = errors.Join(err, fmt.Errorf("rest shutdown: %w", shutdownErr))
	}
	return err
}
