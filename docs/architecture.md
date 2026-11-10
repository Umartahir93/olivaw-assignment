# Architecture

Robots at different sites report their state about once per second. The
system receives that telemetry, stores it, and shows a fleet manager what
the fleet is doing right now and what one robot did recently.

This repository contains three parts:

| Part | Where | Talks | Status |
|---|---|---|---|
| Robot simulator | `cmd/simulator`, `pkg/simulator` | gRPC client | Working |
| Telemetry ingestion (gRPC) | `cmd/fleet`, `pkg/telemetry` | gRPC server, `:9090` | Stub: accepts every request |
| Fleet API (REST) | `cmd/fleet`, `api` | HTTP server, `:8080` | Stub: validates requests, fixed responses |

Both servers run in one binary, `fleet`. Storage is not built yet.

## Overview

```
                                ┌───────────────────────────────────────┐
  Robot simulator               │        fleet binary (cmd/fleet)       │
  (cmd/simulator)               │                                       │
        │                       │   ┌───────────────────────────────┐   │
        │  ReportTelemetry      │   │ gRPC server :9090             │   │
        │  ReportFault          │   │ TelemetryService              │   │
        └──── x-robot-id ──────────▶│ (pkg/telemetry)               │   │
                                │   └───────────────────────────────┘   │
  Admin                         │                                       │
        │  POST   /v1/robots    │   ┌───────────────────────────────┐   │
        │  DELETE /v1/robots/id │   │ REST server :8080             │   │
        └──────────────────────────▶│ Fleet API                     │   │
                                │   │ (api)                         │   │
  Fleet manager / dashboard     │   │                               │   │
        │  GET /v1/robots       │   │                               │   │
        │  GET …/telemetry      │   │                               │   │
        └──── GET …/faults ────────▶│                               │   │
                                │   └───────────────────────────────┘   │
                                └───────────────────┬───────────────────┘
                                                    │ planned
                                                    ▼
                                          ┌───────────────────┐
                                          │ Storage           │
                                          │ (TimescaleDB)     │
                                          └───────────────────┘
```

Two APIs, because there are two kinds of clients:

- **Robots** write: many small messages, every second, from devices. They
  use **gRPC** (`apispecs/grpc/telemetry/v1/telemetry.proto`).
- **People** onboard robots and read the fleet's state, from browsers and
  scripts. They use **REST** (`apispecs/rest/openapi.yaml`).

See [decisions.md](decisions.md) for why.

## Flows

### Onboarding a robot

```
  Admin                         REST API                         Robot
    │                              │                               │
    │  POST /v1/robots             │                               │
    │  {serialNumber, location}    │                               │
    │─────────────────────────────▶│                               │
    │                              │                               │
    │  201 {robotId: <UUID>,       │                               │
    │       status: active,        │                               │
    │       connectivity:          │                               │
    │         never_seen}          │                               │
    │◀─────────────────────────────│                               │
    │                              │                               │
    │  configure robotId on the robot                              │
    │─────────────────────────────────────────────────────────────▶│
    │                              │                               │
    │                              │      from now on: robotId     │
    │                              │      sent as x-robot-id       │
```

The server assigns the robot's ID. The admin only provides the serial
number and where the robot is deployed. Decommissioning
(`DELETE /v1/robots/{robotId}`) keeps the robot and its history; only new
telemetry is no longer accepted.

Today the REST server is a stub and does not store robots, and the
simulator generates its own UUIDs instead of onboarding.

### Reporting telemetry

```
  Robot                                              gRPC server
    │                                                     │
    │   about once per second:                            │
    │   ReportTelemetry {reading} + x-robot-id            │
    │────────────────────────────────────────────────────▶│
    │   OK (or an error code)                             │
    │◀────────────────────────────────────────────────────│
    │                                                     │
    │   when a fault first appears:                       │
    │   ReportFault {fault} + x-robot-id                  │
    │────────────────────────────────────────────────────▶│
    │   OK                                                │
    │◀────────────────────────────────────────────────────│
```

Every reading is a snapshot: battery, position, activity, the codes of the
currently active faults, and when it was taken (`recorded_at`, robot clock).
Fault details (severity, message, time) are sent once through `ReportFault`.

The contract also defines what the robot does when a call fails: retry,
then move the item to a local dead-letter queue and alert. The simulator
does not implement this yet; it logs failed calls and carries on.

### Reading the fleet's state

The fleet manager uses the REST API:

| Endpoint | Answers |
|---|---|
| `GET /v1/robots` | Every robot and its current state; filter by connectivity or status |
| `GET /v1/robots/{robotId}` | One robot: registration, connectivity, latest reading |
| `GET /v1/robots/{robotId}/telemetry` | Readings from a time range of at most 24 hours |
| `GET /v1/robots/{robotId}/faults` | Faults from a time range of at most 24 hours |

Connectivity (`never_seen`, `online`, `stale`, `offline`) is derived by the
server from when it last received data. A robot that is offline cannot say
so itself.

## Code layout

```
apispecs/                 API contracts, the source of truth
  grpc/                   telemetry.proto + buf config (lint, breaking, generate)
  rest/                   openapi.yaml + ogen config
gen/                      generated code, never edited by hand
  telemetry/v1/           gRPC messages, client and server interfaces
  fleet/v1/               REST models, validation, router, server
api/                      REST handlers (implement fleetv1.Handler)
pkg/telemetry/            gRPC handlers (implement TelemetryServiceServer)
pkg/simulator/            robot model
cmd/fleet/                the server binary: wires handlers into gRPC and HTTP servers
cmd/simulator/            the simulator binary
```

The rule: contracts in `apispecs/`, generated code in `gen/`, handwritten
handlers in `api/` and `pkg/`, and wiring (servers, routing, error format)
in `cmd/`.

## Robot simulator

Each simulated robot (`pkg/simulator/robot.go`) lives on a 50 × 50 m map:

- It drives at 1 m/s to a random point, waits, then picks a new one.
- Its battery drains while moving (0.2 %/s) and while idle (0.02 %/s).
- Below 20 % it drives to the charger at the map's origin and charges to
  100 % (1 %/s).

`cmd/simulator` starts N robots, each in its own goroutine, sharing one
gRPC connection. Every interval each robot advances its model and calls
`ReportTelemetry` with its UUID in `x-robot-id`.

```sh
go run ./cmd/fleet
go run ./cmd/simulator -robots 5 -interval 1s -log-level debug
```

## Not built yet

- **Storage** of readings, faults and robots (planned: TimescaleDB).
- **Ingestion logic**: validation, the registry check (`PERMISSION_DENIED`
  for unknown robots) and deduplication.
- **Fleet API logic**: real data, the 24-hour range rule.
- **Simulator**: buffering, retries and dead-letter queue, faults,
  onboarding through REST.
