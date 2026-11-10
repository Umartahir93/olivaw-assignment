package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/umartahir93/olivaw-assignment/migrations"
	"github.com/umartahir93/olivaw-assignment/pkg/gateway"
	gatewayrepository "github.com/umartahir93/olivaw-assignment/pkg/gateway/repository"
	"github.com/umartahir93/olivaw-assignment/pkg/robot"
	robotrepository "github.com/umartahir93/olivaw-assignment/pkg/robot/repository"
)

var (
	host        = flag.String("host", "0.0.0.0", "hostname both servers listen on")
	port        = flag.Int("port", 9090, "gRPC server port")
	restPort    = flag.Int("rest-port", 8080, "REST server port")
	dbURL       = flag.String("database-url", envOr("DATABASE_URL", "postgres://fleet:fleet@localhost:5432/fleet?sslmode=disable"), "PostgreSQL connection URL (env DATABASE_URL)")
	healthCheck = flag.Bool("health-check", false, "check that a running server on -rest-port is healthy, then exit (for container health checks)")
	logLevel    slog.Level
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

	if *healthCheck {
		return checkHealth()
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	// The database must be reachable and its schema up to date before any
	// server accepts requests.
	db, err := migrations.Open(context.Background(), *dbURL)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("close database", "error", closeErr)
		}
	}()
	if err = migrations.Migrate(context.Background(), db); err != nil {
		return err
	}
	log.Info("database migrated")

	gatewayRepo, err := gatewayrepository.New(context.Background(), db)
	if err != nil {
		return err
	}
	defer closeLogged(log, "gateway repository", gatewayRepo)
	robotRepo, err := robotrepository.New(context.Background(), db)
	if err != nil {
		return err
	}
	defer closeLogged(log, "robot repository", robotRepo)
	gateways := gateway.NewService(gatewayRepo)
	robots := robot.NewService(robotRepo)

	grpcAddr := net.JoinHostPort(*host, strconv.Itoa(*port))
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", grpcAddr, err)
	}
	grpcSrv, healthSrv := newGRPCServer(log, gateways, robots)

	restAddr := net.JoinHostPort(*host, strconv.Itoa(*restPort))
	restLis, err := net.Listen("tcp", restAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", restAddr, err)
	}
	restSrv, err := newRESTServer(restAddr, log, db, gateways, robots)
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

	// Graceful shutdown: report NOT_SERVING, stop accepting new requests and
	// let running ones finish within shutdownTimeout.
	log.Info("shutting down")
	healthSrv.Shutdown()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	grpcStopped := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(grpcStopped)
	}()
	if shutdownErr := restSrv.Shutdown(shutdownCtx); shutdownErr != nil {
		err = errors.Join(err, fmt.Errorf("rest shutdown: %w", shutdownErr))
	}
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		// Calls still running after the timeout are cancelled.
		grpcSrv.Stop()
		<-grpcStopped
		err = errors.Join(err, errors.New("grpc shutdown: timed out, remaining calls cancelled"))
	}
	return err
}

// checkHealth asks the running server's health endpoint whether it is
// healthy. The server image has no shell or curl, so the container health
// check runs the server binary itself.
func checkHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	url := "http://" + net.JoinHostPort("localhost", strconv.Itoa(*restPort)) + healthPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	defer func() { _ = res.Body.Close() }() // nothing to do if closing a read body fails
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health check: %s", res.Status)
	}
	return nil
}

// envOr returns the environment variable key, or fallback if it is unset.
func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// closeLogged closes c and logs a failure, for use in defer.
func closeLogged(log *slog.Logger, what string, c io.Closer) {
	if err := c.Close(); err != nil {
		log.Error("close "+what, "error", err)
	}
}
