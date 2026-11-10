// Command simulator runs simulated robots that send telemetry to the gRPC
// server, each about once per interval.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	telemetryv1 "github.com/umartahir93/olivaw-assignment/gen/telemetry/v1"
	"github.com/umartahir93/olivaw-assignment/pkg/simulator"
)

var (
	server   = flag.String("server", "localhost:9090", "gRPC server address")
	robots   = flag.Int("robots", 10, "number of simulated robots")
	interval = flag.Duration("interval", time.Second, "time between two readings of one robot")
	logLevel slog.Level
)

func init() {
	// TextVar parses the level and rejects unknown names during flag.Parse.
	flag.TextVar(&logLevel, "log-level", slog.LevelInfo, "log level: debug, info, warn or error")
}

// How long one call may take before the robot gives up on it.
const callTimeout = 5 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	conn, err := grpc.NewClient(*server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connect to %s: %w", *server, err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Warn("close connection", "error", err)
		}
	}()
	client := telemetryv1.NewTelemetryServiceClient(conn)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("starting robots", "count", *robots, "server", *server, "interval", interval.String())
	var wg sync.WaitGroup
	for range *robots {
		robot := simulator.NewRobot(uuid.NewString(), rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
		wg.Go(func() { runRobot(ctx, client, robot, log) })
	}
	wg.Wait()
	log.Info("stopped")
	return nil
}

// runRobot sends one reading per interval until ctx is cancelled.
func runRobot(ctx context.Context, client telemetryv1.TelemetryServiceClient, robot *simulator.Robot, log *slog.Logger) {
	log = log.With("robot_id", robot.ID)

	// Start at a random point within the first interval, so the robots do
	// not all send at the same moment.
	select {
	case <-ctx.Done():
		return
	case <-time.After(rand.N(*interval)):
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		reading := robot.Step(*interval, time.Now())
		send(ctx, client, robot.ID, reading, log)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func send(ctx context.Context, client telemetryv1.TelemetryServiceClient, robotID string, reading *telemetryv1.TelemetryReading, log *slog.Logger) {
	ctx = metadata.AppendToOutgoingContext(ctx, "x-robot-id", robotID)
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	_, err := client.ReportTelemetry(ctx, &telemetryv1.ReportTelemetryRequest{Reading: reading})
	if err != nil {
		log.Warn("report telemetry failed", "error", err)
		return
	}
	log.Debug("reported telemetry",
		"battery_pct", reading.GetBatteryPct(),
		"activity", reading.GetActivity().String(),
		"x", reading.GetPosition().GetX(),
		"y", reading.GetPosition().GetY(),
	)
}
