// Command simulator is a virtual gateway: it scans its simulated robots,
// reports them to the system and sends one measurement per robot every
// interval. Once started it keeps sending until it is stopped: a failed
// measurement does not stop it, and the startup scan is retried until the
// server answers.
//
// It is only the gateway: registering it is an operator's job over the
// REST API, done before the simulator starts; the simulator gets the
// resulting gatewayId with -gateway-id. See README.md.
//
// It was written with the least effort, as part of the assignment: it only
// has to drive the server with realistic traffic, and the server side is
// the focus. It is not a reference implementation of the gateway contract
// in apispecs/grpc. What it leaves out:
//   - faults: it never calls ReportFault and reports no active fault codes;
//   - delivery rules (docs/decisions.md, 11): a failed measurement is
//     logged and dropped, with no buffer, retries or dead-letter queue;
//   - rescans: it scans once at startup; robots never join, leave or go
//     silent, so none of them ever turns offline.
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	gatewayv1 "github.com/umartahir93/olivaw-assignment/gen/gateway/v1"
	gatewaygrpc "github.com/umartahir93/olivaw-assignment/pkg/gateway/grpc"
	"github.com/umartahir93/olivaw-assignment/pkg/simulator"
)

var (
	server    = flag.String("server", "localhost:9090", "gRPC server address")
	gatewayID = flag.String("gateway-id", "", "gatewayId of the gateway, from its registration over REST (required)")
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

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))

	// Registering the gateway is an operator's job over REST (POST
	// /v1/gateways, or `make register-gateway`); the simulator is only the
	// gateway and starts from the gatewayId it was given.
	if _, err := uuid.Parse(*gatewayID); err != nil {
		return errors.New("-gateway-id is required: register the gateway over REST first (make register-gateway) and pass its gatewayId")
	}
	log = log.With("gateway_id", *gatewayID)

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
	ctx = metadata.AppendToOutgoingContext(ctx, gatewaygrpc.GatewayIDKey, *gatewayID)

	fleet := make([]*simulator.Robot, *robots)
	for i := range fleet {
		fleet[i] = simulator.NewRobot(robotSerial(*gatewayID, i+1), rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	}
	if err := scanUntilDone(ctx, gateways, fleet, log); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return nil // stopped before the first scan went through
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

// Backoff between two scan attempts: it doubles after every failure, up to
// maxBackoff.
const (
	firstBackoff = time.Second
	maxBackoff   = 30 * time.Second
)

// scanUntilDone repeats scan until it succeeds, so a simulator started
// before the server, or during an outage, starts sending once the server is
// back. Errors a retry cannot fix (INVALID_ARGUMENT, UNAUTHENTICATED) end
// it; so does ctx.
func scanUntilDone(ctx context.Context, client gatewayv1.GatewayServiceClient, fleet []*simulator.Robot, log *slog.Logger) error {
	backoff := firstBackoff
	for {
		err := scan(ctx, client, fleet, log)
		switch status.Code(err) {
		case codes.OK:
			return nil
		case codes.InvalidArgument, codes.Unauthenticated:
			return err
		}
		log.Warn("scan failed, retrying", "error", err, "retry_in", backoff.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, maxBackoff)
	}
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
