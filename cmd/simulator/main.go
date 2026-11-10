// Command simulator is a virtual gateway: it scans its simulated robots,
// reports them to the system and sends one measurement per robot every
// interval.
package main

import (
	"context"
	"errors"
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

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	"github.com/umartahir93/olivaw-assignment/pkg/simulator"
)

var (
	server    = flag.String("server", "localhost:9090", "gRPC server address")
	gatewayID = flag.String("gateway-id", uuid.NewString(), "gatewayId assigned at registration (default: a random one)")
	robots    = flag.Int("robots", 10, "number of simulated robots")
	interval  = flag.Duration("interval", time.Second, "time between two measurements of one robot")
	logLevel  slog.Level
)

func init() {
	// TextVar parses the level and rejects unknown names during flag.Parse.
	flag.TextVar(&logLevel, "log-level", slog.LevelInfo, "log level: debug, info, warn or error")
}

// How long one call may take before the gateway gives up on it.
const callTimeout = 5 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})).With("gateway_id", *gatewayID)

	conn, err := grpc.NewClient(*server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connect to %s: %w", *server, err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Warn("close connection", "error", err)
		}
	}()
	gateways := gatewayv1.NewGatewayServiceClient(conn)
	robotsClient := gatewayv1.NewRobotServiceClient(conn)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Every call identifies the gateway.
	ctx = metadata.AppendToOutgoingContext(ctx, "x-gateway-id", *gatewayID)

	fleet := make([]*simulator.Robot, *robots)
	for i := range fleet {
		fleet[i] = simulator.NewRobot(fmt.Sprintf("SIM-%03d", i+1), rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	}
	if err := scan(ctx, gateways, fleet, log); err != nil {
		return err
	}

	log.Info("sending measurements", "robots", len(fleet), "server", *server, "interval", interval.String())
	var wg sync.WaitGroup
	for _, robot := range fleet {
		wg.Go(func() { runRobot(ctx, robotsClient, robot, log) })
	}
	wg.Wait()
	log.Info("stopped")
	return nil
}

// scan asks the system which robots it already knows for this gateway,
// reports the full robot list and stores the robotId it gets back for
// each robot.
func scan(ctx context.Context, client gatewayv1.GatewayServiceClient, fleet []*simulator.Robot, log *slog.Logger) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	known, err := client.ListRobots(callCtx, &gatewayv1.ListRobotsRequest{})
	if err != nil {
		return fmt.Errorf("list robots: %w", err)
	}
	log.Info("robots known to the system", "count", len(known.GetRobots()))

	serials := make([]string, len(fleet))
	for i, robot := range fleet {
		serials[i] = robot.SerialNumber
	}
	callCtx, cancel = context.WithTimeout(ctx, callTimeout)
	defer cancel()
	scanned, err := client.ReportScan(callCtx, &gatewayv1.ReportScanRequest{SerialNumbers: serials})
	if err != nil {
		return fmt.Errorf("report scan: %w", err)
	}

	ids := make(map[string]string, len(scanned.GetRobots()))
	for _, r := range scanned.GetRobots() {
		ids[r.GetSerialNumber()] = r.GetRobotId()
	}
	for _, robot := range fleet {
		robot.ID = ids[robot.SerialNumber]
		if robot.ID == "" {
			return errors.New("report scan: no robotId for " + robot.SerialNumber)
		}
	}
	log.Info("scan reported", "robots", len(fleet))
	return nil
}

// runRobot sends one measurement per interval until ctx is cancelled.
func runRobot(ctx context.Context, client gatewayv1.RobotServiceClient, robot *simulator.Robot, log *slog.Logger) {
	log = log.With("robot_id", robot.ID, "serial_number", robot.SerialNumber)

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
		send(ctx, client, robot.Step(*interval, time.Now()), log)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func send(ctx context.Context, client gatewayv1.RobotServiceClient, m *gatewayv1.Measurement, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	_, err := client.ReportMeasurement(ctx, &gatewayv1.ReportMeasurementRequest{Measurement: m})
	if err != nil {
		log.Warn("report measurement failed", "error", err)
		return
	}
	log.Debug("reported measurement",
		"battery_pct", m.GetBatteryPct(),
		"activity", m.GetActivity().String(),
		"x", m.GetPosition().GetX(),
		"y", m.GetPosition().GetY(),
	)
}
