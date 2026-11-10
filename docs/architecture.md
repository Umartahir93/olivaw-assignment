# Architecture

Robots at different sites report their state about once per second, but
not to the system directly: each site has a **gateway**, an edge device
that discovers the site's robots, collects their measurements and forwards
them. The system stores those measurements and shows a fleet manager what
the fleet is doing right now and what one robot did recently. It only ever
talks to gateways (over gRPC) and to people (over REST).

## Terms

These words mean the same thing everywhere in this project: in the docs,
the API contracts and the code.

| Term | Meaning |
|---|---|
| **Gateway** | The concept: an edge device at a site that robots are connected to. It is the only thing that sends data to the system. |
| **Simulator** | A virtual gateway (`cmd/simulator`): software that plays the role of a gateway, with simulated robots behind it. It is how the gateway concept is realised in this project, so **"gateway" and "simulator" are used interchangeably**. |
| **Robot** | A machine connected to a gateway. It never talks to the system itself; its gateway reports for it. |
| **Measurement** | A snapshot of one robot's state at one moment: battery, position, activity and active fault codes. |
| **Fault** | A problem a robot detected, reported once when it first appears. |
| **Serial number** | The ID a gateway or robot already has from its manufacturer. It is the **business key**: how people and gateways recognise a gateway or robot from the outside. |
| **`gatewayId`, `robotId`** | The **identity** the system gives a gateway or robot (a UUID). It is assigned once and **never changes during the gateway's or robot's lifecycle**. Everything inside the system refers to these. |
| **Scan** | The gateway finding out which robots are connected to it and sending the system that list. For the sake of simplicity, a scan is just this list of robots sent by the simulator: the server does not keep any record of scans. |
| **Connectivity** | Whether the system hears from a gateway or robot: `unknown`, `online` or `offline`. Derived by the system, never reported. |
| **Admin** | The person who registers gateways. |
| **Fleet manager** | The person who watches the fleet: gateways, robots, measurements and faults. |
| **System** | Everything this repository runs on the server side: the gRPC and REST servers, the service layer and storage. |

## Overview

```
                                                           fleet binary (cmd/fleet)
 (1) Admin registers the gateway (simulator)              ┌──────────────────────────┐
     │  POST /v1/gateways {serialNumber, location}        │                          │
     ├───────────────────────────────────────────────────▶│ REST server :8080        │
     │◀── 201 {gatewayId} ────────────────────────────────│ Fleet API                │
     │                                                    │                          │
     │ (2) configure gatewayId on the gateway             │                          │
     ▼                                                    │                          │
┌────────────────────────────┐                            │                          │
│ Site "Munich HQ"           │                            │                          │
│                            │ (3) ListRobots, ReportScan │                          │
│  Robot ──┐                 ├───────────────────────────▶│ gRPC server :9090        │
│  Robot ──┼──▶ Gateway      │                            │   GatewayService         │
│  Robot ──┘    (simulator)  │ (4) ReportMeasurement,     │   RobotService           │
│               x-gateway-id │     ReportFault            │                          │
│                            ├───────────────────────────▶│                          │
└────────────────────────────┘                            │                          │
                                                          │                          │
 (5) Fleet manager                                        │                          │
     │  GET gateways, robots, measurements, faults        │                          │
     └───────────────────────────────────────────────────▶│ (same REST server)       │
                                                          └─────────────┬────────────┘
                                                                        │
                                                                        ▼
                                                              ┌───────────────────┐
                                                              │ Storage (planned: │
                                                              │ TimescaleDB)      │
                                                              └───────────────────┘
```

The numbers follow the order of the [lifecycle](#lifecycle): register the
gateway, configure its ID, scan, report measurements, read the fleet.

Two APIs, because there are two kinds of clients:

- **Gateways** write: robot lists and a steady stream of measurements, from
  edge devices. They use **gRPC** (`apispecs/grpc`).
- **People** register gateways and read the fleet's state, from browsers and
  scripts. They use **REST** (`apispecs/rest`).

See [decisions.md](decisions.md) for why.

## Domain

| Entity | Fields | Created by |
|---|---|---|
| **Gateway** | `gatewayId`, `serialNumber`, `location`, `name` (optional), `registeredAt`, `connectivity`, `lastSeenAt` | Admin, through REST |
| **Robot** | `robotId`, `serialNumber`, `gatewayId`, `firstSeenAt`, `connectivity`, `lastSeenAt`, latest measurement | The gateway, through its robot list |
| **Measurement** | `robotId`, battery, position, activity, active fault codes, `measuredAt`, `receivedAt` | The robot; shared via its gateway (edge device) |
| **Fault** | `robotId`, `code`, `severity`, `message`, `occurredAt` | The robot; shared via its gateway (edge device) |

```
 Gateway 1 ──── n Robot 1 ──── n Measurement
                        1 ──── n Fault
```

A gateway has many robots; a robot has many measurements and many faults.

## Lifecycle

```
 Admin             REST API              Gateway                gRPC API
   │                  │                     │                       │
   │ POST /v1/gateways│                     │                       │
   │ {serialNumber,   │                     │                       │
   │  location, name?}│                     │                       │
   │─────────────────▶│                     │                       │
   │ 201 {gatewayId}  │                     │                       │
   │◀─────────────────│                     │                       │
   │                  │                     │                       │
   │ configure gatewayId on the gateway     │                       │
   │───────────────────────────────────────▶│                       │
   │                  │                     │                       │
   │                  │   on start:         │ ListRobots()          │
   │                  │                     │──────────────────────▶│
   │                  │                     │ robots known so far   │
   │                  │                     │◀──────────────────────│
   │                  │                     │                       │
   │                  │   after a scan:     │ ReportScan(full list) │
   │                  │                     │──────────────────────▶│
   │                  │                     │ robotId per robot     │
   │                  │                     │◀──────────────────────│
   │                  │                     │                       │
   │                  │   per robot,        │ ReportMeasurement(…)  │
   │                  │   about once/second │──────────────────────▶│
   │                  │                     │ OK                    │
   │                  │                     │◀──────────────────────│
   │                  │                     │                       │
   │                  │   when a fault      │ ReportFault(…)        │
   │                  │   first appears     │──────────────────────▶│
```

1. **Register the gateway.** An admin registers it with its serial number
   and location. The system returns its `gatewayId`, which is configured on
   the gateway. The gateway sends it as `x-gateway-id` on every gRPC call
   (in production: its client certificate).
2. **Discover robots.** The gateway scans its local network and sends the
   **full list** of its robots' serial numbers. New robots get a `robotId`;
   robots already known keep theirs. After a restart, the gateway asks for
   the robots the system already knows, so every robot keeps its `robotId`.
3. **Report measurements.** About once per second per robot, one
   measurement per request, with the robot's `robotId`.
4. **Report faults.** Once, when a fault first appears. While it is active,
   its code is listed in every measurement; it is resolved when the code
   disappears.

A gateway may only report for its own robots.

For the sake of simplicity, decommissioning a gateway and deleting a robot
are not part of the lifecycle: once registered or discovered, gateways and
robots stay. A robot missing from a later scan is kept as it is.

## Rules

- **IDs** (`gatewayId`, `robotId`) are UUIDs assigned by the system. Serial
  numbers come from the outside world.
- **A robot belongs to exactly one gateway** and never moves to another one.
- **Connectivity** is derived by the system, never reported:
  `unknown` until the first contact, `online` if something arrived in the
  last 5 minutes, otherwise `offline`.
  - A robot counts as seen when one of its measurements arrives.
  - A gateway counts as seen on any gRPC call it makes.

  A gateway that is offline means the whole site is dark; a single robot
  that is offline means that robot stopped reporting.

## APIs

### gRPC (gateways)

Every call carries `x-gateway-id`.

| Service | Method | Purpose |
|---|---|---|
| `GatewayService` | `ListRobots` | Robots the system knows for this gateway, with their `robotId` |
| `GatewayService` | `ReportScan` | Full list of the gateway's robots after a scan; returns a `robotId` per robot |
| `RobotService` | `ReportMeasurement` | One measurement of one robot |
| `RobotService` | `ReportFault` | One fault of one robot |

**Delivery.** The gateway buffers what it could not deliver and sends it
oldest first.

| Response | Gateway does |
|---|---|
| OK | Delete the item from its buffer |
| `INVALID_ARGUMENT`, `UNAUTHENTICATED` | Drop it and log locally; resending fails the same way |
| Anything else | Retry up to 3 times with exponential backoff, then move it to a local dead-letter queue, log it and count it in a metric; an alert brings in an operator |

Resending is safe because every item has an **idempotency key**:
`(robotId, measuredAt)` for a measurement, `(robotId, code, occurredAt)`
for a fault. An item whose key is already stored is answered with OK and
not stored again, so sending it twice has the same effect as sending it
once.

### REST (people)

| Endpoint | Purpose |
|---|---|
| `POST /v1/gateways` | Register a gateway |
| `GET /v1/gateways` | List gateways |
| `GET /v1/gateways/{gatewayId}` | One gateway |
| `GET /v1/gateways/{gatewayId}/robots` | A gateway's robots and their current state |
| `GET /v1/robots/{robotId}` | One robot and its current state |
| `GET /v1/robots/{robotId}/measurements` | A robot's measurement history |
| `GET /v1/robots/{robotId}/faults` | A robot's fault history |

- **Gateways** come back with their serial number, location, name,
  connectivity and when they were last seen.
- **A gateway's robots** are all robots found by its scans. For each robot:
  `robotId`, `serialNumber`, connectivity, when it was last seen, and its
  latest measurement (battery, position, activity, active fault codes). This
  is the view of one site: what every robot there is doing right now.
- **Measurement history** returns one robot's measurements oldest first,
  each with battery, position, activity, active fault codes, `measuredAt`
  and `receivedAt`. It is the history behind a robot's charts, e.g. battery
  over the day.
- **Fault history** returns the faults one robot reported, oldest first,
  each with `code`, `severity`, `message` and `occurredAt`: what went wrong
  and when. Which faults are active right now is in the latest measurement.
- **Time range** for both histories: `from` and `to` can be on any date but
  span at most 24 hours; without them, the last 24 hours.

Robots cannot be created through REST; only their gateway reports them.
Lists use `offset` and `limit` and return `hasMore`. Errors use problem
details (RFC 9457).

## Code layout

```
apispecs/                 API contracts, the source of truth
  grpc/                   proto + buf config (lint, breaking, generate)
  rest/                   openapi.yaml + ogen config
gen/                      generated code, never edited by hand
api/                      REST handlers (implement the generated fleetv1.Handler)
pkg/telemetry/            gateway-facing logic
  domain/                 entities and their rules
  service.go              service layer: rules, uses a repository
  adapter/                gRPC handlers in, storage out
pkg/simulator/            robot model used by the gateway simulator
cmd/fleet/                the server binary: wires handlers into gRPC and HTTP servers
cmd/simulator/            the gateway simulator
```

The rule: contracts in `apispecs/`, generated code in `gen/`, handwritten
logic in `api/` and `pkg/`, wiring (servers, routing, error format) in
`cmd/`. Within `pkg/telemetry`, the domain and the service know nothing
about gRPC or storage; adapters translate.

## Gateway simulator

The simulator (`cmd/simulator`) is one gateway with a few robots behind it.
It asks the system which robots it already knows, sends its robot list,
and then sends one measurement per robot every second.

The robots (`pkg/simulator`) drive around a small map, use up their
battery, and go back to a charger when the battery runs low.

## Future: an event-driven pipeline

For the sake of simplicity, the gateway (here: the simulator) talks
directly to the service, and every measurement is handled within the
gRPC call that delivers it: validated, stored, answered. There is no
message broker or pipeline in between, and the design is not event-driven.

```
 Today:    Gateway ──gRPC──▶ service ──▶ storage          (one synchronous call)
```

This handles the load of this project, but it will not scale well:

- **Ingestion is tied to storage.** Every call waits for the database. When
  storage slows down, calls slow down, gateways' buffers fill and the
  system can only push back.
- **Spikes hit the database directly.** When many gateways reconnect after
  an outage, their backlogs arrive at the same time.
- **One consumer only.** Anything else that needs measurements (alerting,
  connectivity tracking, analytics) has to be built into the same call or
  read from the database afterwards.

The direction to grow in is an event-driven pipeline: ingestion only
accepts and publishes each measurement as an event; independent consumers
process it at their own pace.

```
 Future:   Gateway ──gRPC/MQTT──▶ ingestion ──▶ broker (e.g. Kafka, NATS JetStream)
                                                   │
                                   ┌───────────────┼───────────────┐
                                   ▼               ▼               ▼
                             storage writer   connectivity     alerting
                             (batched)        tracker
```

The contracts already fit this: a gateway gets OK once a measurement is
accepted, and measurements carry their own key, so a consumer can handle a
redelivered event without storing it twice.
