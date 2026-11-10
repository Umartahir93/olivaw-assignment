# Olivaw fleet management: server side

Robots at different sites report their state about once per second, but
not to the cloud directly: each site has a **gateway**, an edge device
that discovers the site's robots, collects their telemetry and forwards
it. This repository is **the system** on the other end, the cloud
service: it ingests that telemetry, keeps track of every gateway and
robot, and shows a fleet manager what the fleet is doing right now.
There is no real hardware here: the **simulator** plays a gateway with a
few simulated robots.

```
   Site "Munich HQ"                          The system (this repository)
  ┌──────────────────────────────┐          ┌─────────────────────────────┐
  │ Robot ─┐                     │   gRPC   │ gRPC :9090  ◀── gateways    │
  │ Robot ─┼─▶ Gateway ──────────┼─────────▶│                             │
  │ Robot ─┘   (simulator)       │          │ services: business rules    │──▶ PostgreSQL
  └──────────────────────────────┘          │                             │    + TimescaleDB
                                            │ REST :8080  ◀── people      │
   Admin, fleet manager ─────────── REST ──▶│                             │
                                            └─────────────────────────────┘
```

## What it does

The system has two APIs, one per kind of client:

- **gRPC for gateways (machines):** typed, compact and generated from the
  contract in [`apispecs/grpc`](apispecs/grpc).
- **REST for people (admins, fleet managers):** easy to use from a
  browser or curl, generated from [`apispecs/rest/openapi.yaml`](apispecs/rest/openapi.yaml).

A gateway's lifecycle:

```
 Registration ──────────▶ Discovery ─────────────▶ Ingestion ─────────────▶ Monitoring
 admin, REST             gateway, gRPC            gateway, gRPC            fleet manager, REST
 → gatewayId             → robotIds               → measurements, faults   → state, history
```

1. **Registration.** An admin registers the gateway over REST and gets
   its `gatewayId`, which the gateway sends with every gRPC call.
2. **Discovery.** The gateway reports the robots it found (a scan); each
   new robot gets a `robotId` that never changes.
3. **Ingestion.** The gateway sends one measurement per robot (battery,
   position, activity, active fault codes) about once per second, and a
   fault whenever a robot detects one. The system validates and stores
   both.
4. **Monitoring.** A fleet manager reads gateways, robots, each robot's
   latest state, its measurement history and its faults over REST.
   Connectivity is derived, never reported: `unknown` until the first
   data arrives, `online` while it keeps arriving, `offline` after 5
   minutes of silence.

> [!NOTE]
> Robots never call the system, and people never call gateways. Each
> client uses only its own API, so a change to the REST API never touches
> a gateway, and the other way round.

## Measurement storage

Everything lives in one **PostgreSQL** database with the **TimescaleDB**
extension. Gateways, robots and faults are plain tables.

Measurements are different. Each one is a snapshot of a robot's state;
they are **append-only** (written once, never changed), arrive about once
per second per robot, and are always read the same way: one robot over a
time range of at most 24 hours. So they go into a TimescaleDB
**hypertable**, partitioned by time into chunks: a read touches only the
chunks of its range, however long the history grows.

```
 measurements (hypertable, partitioned by measured_at)
 ┌──────────────┬──────────────┬──────────────┬──────────────┐
 │ chunk: week 1│ chunk: week 2│ chunk: week 3│ chunk: now   │ ◀── inserts
 └──────────────┴──────────────┴──────────────┴──────┬───────┘
                                                     │
              "robot X, the last 24 hours" reads only the newest chunk
```

- **Idempotent writes.** A measurement is keyed by `(robotId, measuredAt)`
  and a fault by `(robotId, code, occurredAt)`. A gateway can resend after
  a timeout; a duplicate is acknowledged and not stored twice.
- **Two clocks.** `measuredAt` is the robot's time; the system records
  `receivedAt` itself, and connectivity is based on it, since device clocks
  can drift.
- **No write amplification.** A scan marks a robot as seen; after that,
  its latest state and `lastSeenAt` come from its newest measurement at
  read time, so ingesting a measurement is a single insert.

> [!TIP]
> A robot goes `offline` on its own: once its newest measurement is more
> than 5 minutes old, the next read reports it. No background job is
> needed.

## Requirements

- **Go 1.26**, to build and run the tests locally
- **Docker** with **Compose** (`docker compose` or `docker-compose`)
- **make**, **curl** and **jq** (to format the JSON answers in "Try it out")

Dependencies are vendored; code generation and lint tools install into
`./bin` on first use.

## Try it out

The steps below walk through a gateway's lifecycle in a few minutes. The
API contracts ([gRPC](apispecs/grpc), [REST](apispecs/rest/openapi.yaml))
have every endpoint and parameter.

> [!TIP]
> Run the steps in one shell: each step keeps the IDs it needs in a
> variable. Registering is an admin's job, so it uses curl; the gateway
> talks gRPC, so the simulator is started with make.

### 1. Start the database and server
Start PostgreSQL and the application (REST on `:8080`, gRPC on `:9090`).

```sh
make up
```

### 2. Register a gateway
```sh
GATEWAY_ID=$(curl -s -X POST localhost:8080/v1/gateways \
  -H 'Content-Type: application/json' \
  -d "{\"serialNumber\": \"GW-$(date +%s)\",\"location\": \"Munich HQ\"}" \
  | jq -r '.gatewayId')

echo "Gateway ID: $GATEWAY_ID"
```

### 3. Start the gateway simulator
The simulator runs in the background: it reports its robots (a scan),
then sends one measurement per robot every second.

```sh
make simulate GATEWAY_ID=$GATEWAY_ID
```

### 4. Get gateway details
```sh
curl -s localhost:8080/v1/gateways/$GATEWAY_ID | jq .
```

### 5. List the gateway's robots
```sh
curl -s localhost:8080/v1/gateways/$GATEWAY_ID/robots | jq .
```

### 6. Get a robot ID
```sh
ROBOT_ID=$(curl -s localhost:8080/v1/gateways/$GATEWAY_ID/robots | jq -r '.items[0].robotId')
echo "Robot ID: $ROBOT_ID"
```

### 7. Get the robot's measurements
Without `from` and `to`, this returns the last 24 hours; a range can be
at most 24 hours long. `/v1/robots/$ROBOT_ID/faults` works the same way.

```sh
curl -s localhost:8080/v1/robots/$ROBOT_ID/measurements | jq .
```

### 8. Stop everything
```sh
make down
```

All other commands (tests, lint, code generation, Docker): run `make help`.

## What I built, faked and left out

**Built: the server side.** It is where the interesting trade-offs of this
problem are: the contracts, gateway identity, ingestion, idempotent
storage, connectivity and failure handling. All of it is done end to end
and tested with unit, integration and end-to-end tests, rather than three
half-finished parts.

**Faked or left out, and why that is the right trade:**

- **The gateway** is the simulator, written with the least effort: it only
  has to drive the server with realistic traffic
  ([`cmd/simulator/README.md`](cmd/simulator/README.md)).
- **The user interface** is left out: the REST API is what a UI would use,
  and it is fully specified and tested.
- **Authentication** is faked: a gateway sends its `gatewayId` in the
  `x-gateway-id` metadata instead of a client certificate, and REST has no
  login. Real identity does not change the data flow, which is what this
  project is about.

## Current limitations and how to extend

Each of these keeps the project small. The extensions add to the APIs
rather than change them; only real identity changes how a gateway
connects (a client certificate instead of `x-gateway-id`).

| Limitation | Today | How to extend |
|---|---|---|
| **Synchronous ingestion** | Each measurement is validated and stored within its gRPC call; no broker ([decision 15](docs/decisions.md#15-direct-calls-no-pipeline)). | Ingestion only validates and publishes to a broker (Kafka, NATS JetStream); consumers store in batches, track connectivity and alert. Ingestion no longer waits for the database, and backlogs after an outage are absorbed ([event-driven pipeline](docs/architecture.md#future-an-event-driven-pipeline)). |
| **No live view** | The fleet manager polls REST. | A live endpoint (server-sent events, WebSocket, or gRPC server streaming) that pushes new measurements and connectivity changes to dashboards, fed by the broker. |
| **Limited history** | Raw measurements, at most 24 hours per request, every item in one response ([decision 13](docs/decisions.md#13-history-24-hours-per-request-no-pagination)). | A historical endpoint: aggregates such as per-minute averages from TimescaleDB continuous aggregates, longer ranges, cursor pagination and export; a column store such as ClickHouse for fleet-wide analytics. |
| **No compression or retention** | Every measurement is kept uncompressed ([decision 17](docs/decisions.md#17-storage-timescaledb)). | TimescaleDB compression for old chunks, and a retention policy once the business decides how long raw data matters. |
| **Faked identity** | `x-gateway-id` metadata; REST has no auth ([decision 7](docs/decisions.md#7-gateway-identity-in-metadata)). | Mutual TLS for gateways; authentication and roles (admin, fleet manager) for people. |
| **One measurement per call** | Unary gRPC calls ([decision 5](docs/decisions.md#5-unary-calls-one-measurement-per-request)). | Batched requests or client streaming for sites with many robots. |
| **Gateway delivery rules** | The simulator drops a failed measurement: no buffer, no retry, and it sends no faults. | A real gateway client with the rules the contract describes: buffer, retry with backoff, dead-letter queue ([decision 11](docs/decisions.md#11-delivery-buffer-retry-dead-letter-queue)). |
| **Observability** | Structured logs and health checks only. | Metrics (ingestion rate, latency, errors) and distributed tracing next to the logs. |
| **No alerting** | Someone has to watch the REST API to notice a problem. | Alerts (email, Slack, webhook) when a robot goes `offline`, reports a critical fault or runs low on battery, raised by a consumer of the broker. |
| **No fleet-wide view** | Reads are per gateway or per robot. | Fleet-wide endpoints: robots per connectivity and activity, battery distribution, faults per site, built on TimescaleDB aggregates. |
| **One server instance** | One process serves both APIs; nothing limits a single gateway. | The server is stateless, so it scales out behind a load balancer, with PgBouncer for connections and read replicas for REST reads; rate limits and backpressure per gateway, so one site cannot flood ingestion. |
| **No CI/CD or load tests** | Tests, lint and the proto breaking-change check run locally with make. | A pipeline running them on every pull request, plus load and soak tests (ghz for gRPC, k6 for REST) to find where synchronous ingestion stops keeping up. |

## Architecture and decisions

> [!IMPORTANT]
> The complete architecture is explained in
> [`docs/architecture.md`](docs/architecture.md), and every decision
> behind it, with its reasons, its costs and when to revisit it, in
> [`docs/decisions.md`](docs/decisions.md). Start with the architecture.

**[`docs/architecture.md`](docs/architecture.md): how the system works**

| Section | What it explains |
|---|---|
| [Terms](docs/architecture.md#terms) | the words used everywhere, from gateway to connectivity |
| [Overview](docs/architecture.md#overview) | gateways, the system and people in one picture |
| [Domain](docs/architecture.md#domain) and [Rules](docs/architecture.md#rules) | the entities, how they relate, and the rules they follow |
| [Lifecycle](docs/architecture.md#lifecycle) | from registering a gateway to its robots' measurements |
| [APIs](docs/architecture.md#apis) | the gRPC API for gateways and the REST API for people |
| [Code layout](docs/architecture.md#code-layout) | where everything lives, and the layers of each package |
| [Design principles](docs/architecture.md#design-principles) | the ideas the code follows |
| [What is built, faked and left out](docs/architecture.md#what-is-built-faked-and-left-out) | the scope of this project |
| [Future: an event-driven pipeline](docs/architecture.md#future-an-event-driven-pipeline) | how ingestion would grow at production load |

**[`docs/decisions.md`](docs/decisions.md): why it works this way**

| Area | Decisions |
|---|---|
| Shape of the system | [1. Robots report through gateways](docs/decisions.md#1-robots-report-through-gateways) · [2. gRPC for gateways, REST for people](docs/decisions.md#2-grpc-for-gateways-rest-for-people) · [3. API-first, with generated code](docs/decisions.md#3-api-first-with-generated-code) · [16. One binary for both servers](docs/decisions.md#16-one-binary-for-both-servers) |
| Gateway API | [4. Two gRPC services, split by intent](docs/decisions.md#4-two-grpc-services-split-by-intent) · [5. Unary calls, one measurement per request](docs/decisions.md#5-unary-calls-one-measurement-per-request) · [6. Faults: one event, plus codes in every measurement](docs/decisions.md#6-faults-one-event-plus-codes-in-every-measurement) · [7. Gateway identity in metadata](docs/decisions.md#7-gateway-identity-in-metadata) |
| Identity and data | [8. Serial number as business key, UUID as identity](docs/decisions.md#8-serial-number-as-business-key-uuid-as-identity) · [9. No decommissioning, no deleting](docs/decisions.md#9-no-decommissioning-no-deleting) · [10. Idempotency keys instead of sequence numbers](docs/decisions.md#10-idempotency-keys-instead-of-sequence-numbers) |
| Delivery and state | [11. Delivery: buffer, retry, dead-letter queue](docs/decisions.md#11-delivery-buffer-retry-dead-letter-queue) · [12. Connectivity derived by the system](docs/decisions.md#12-connectivity-derived-by-the-system) |
| Reading and errors | [13. History: 24 hours per request, no pagination](docs/decisions.md#13-history-24-hours-per-request-no-pagination) · [14. Errors as problem details](docs/decisions.md#14-errors-as-problem-details) |
| Ingestion and storage | [15. Direct calls, no pipeline](docs/decisions.md#15-direct-calls-no-pipeline) · [17. Storage: TimescaleDB](docs/decisions.md#17-storage-timescaledb) |
