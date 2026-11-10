# CLAUDE.md

Guidance for Claude Code sessions in this repository.

## The project

Server side of the Olivaw engineering challenge (Go): a fleet management
system. Robots report through a **gateway** (edge device) over **gRPC**;
people (admin, fleet manager) use **REST**. The **simulator** is a fake
gateway. Measurements are stored in PostgreSQL + TimescaleDB.

Read these first, in this order:
1. `README.md`: what is built, faked and left out, limitations, how to run it.
2. `docs/architecture.md`: the complete architecture (terms, domain, rules,
   lifecycle, APIs, code layout).
3. `docs/decisions.md`: 17 numbered decisions with reasons and costs.

The terms in `docs/architecture.md#terms` (system, gateway, simulator,
admin, fleet manager, measurement, fault, connectivity, gatewayId/robotId)
are used the same way in docs, contracts and code. Keep it that way.

## Where things live

| Path | What |
|---|---|
| `apispecs/grpc/gateway/v1/*.proto` | gRPC contract (buf); source of truth |
| `apispecs/rest/openapi.yaml` | REST contract (ogen); source of truth |
| `gen/` | generated code (`make generate`); never edit by hand |
| `cmd/fleet/` | the server: wiring, gRPC + REST servers, health, shutdown |
| `cmd/simulator/` | the fake gateway (its own `README.md`) |
| `pkg/gateway/`, `pkg/robot/` | one package per entity: `domain/`, `service.go` (Repository interface + Service), `repo_fake.go`, `repository/` (sqlx), `grpc/`, `handler/` (REST) |
| `middleware/` | logging + panic recovery for gRPC and REST |
| `migrations/` | goose SQL migrations (embedded) and the DB connection |
| `utils/` | small helpers (DB error checks) |
| `test/e2e/` | end-to-end tests against the running system |
| `test/postgres/` | test database setup (SQL for the container, Go helper) |
| `vendor/` | vendored dependencies (`make vendor` after changing go.mod) |

## Database

PostgreSQL with the TimescaleDB extension, migrated by goose at server
start (`migrations/`, embedded in the binary). Two databases in Docker:
`fleet` for the server (`DATABASE_URL`) and `fleet_test` for integration
tests (`TEST_DATABASE_URL`; created by `test/postgres/01-test-database.sql`,
emptied before every test case).

| Table | Kind | Key | Notes |
|---|---|---|---|
| `gateways` | plain | `id` (uuid) | `serial_number` unique; `last_seen_at` set on every gRPC call from the gateway |
| `robots` | plain | `id` (uuid) | `serial_number` unique; `gateway_id` → `gateways`; `last_seen_at` set when a scan reports the robot; index `(gateway_id, first_seen_at)` |
| `measurements` | **TimescaleDB hypertable** on `measured_at` | `(robot_id, measured_at)` | `robot_id` → `robots`; `received_at` is the server's clock; position columns all null when not reported; `active_fault_codes text[]` |
| `faults` | plain | `(robot_id, code, occurred_at)` | `robot_id` → `robots`; index `(robot_id, occurred_at)` |

- The primary keys of `measurements` and `faults` are the contract's
  idempotency keys: inserts use `ON CONFLICT DO NOTHING`, so a resent item
  is stored once.
- A robot's `lastSeenAt` and `latest` measurement are not stored on the
  robot: robot reads join each robot to its newest measurement
  (`LEFT JOIN LATERAL`), and `lastSeenAt` is the later of that
  measurement's `received_at` and `robots.last_seen_at`. Connectivity
  (`unknown` / `online` within 5 minutes / `offline`) is derived from it
  at read time.
- Telemetry is append-only: nothing updates or deletes measurements or
  faults. No compression or retention policy is set up yet.
- A schema change is a new numbered migration in `migrations/`; never edit
  an applied one.

## Commands

`make help` lists everything. The ones used most:

| Command | What |
|---|---|
| `make test_unit` | unit tests (`//go:build unit`) |
| `make integration_test` | integration tests against the test DB (`//go:build integration`); starts the DB |
| `make test_e2e_docker` | rebuilds the server image, runs e2e tests (`//go:build e2e`) in Docker |
| `make lint-go` | golangci-lint over all build tags |
| `make generate` | regenerate `gen/` after a contract change |
| `make up` / `make down` | database + server in Docker |

Before calling work done: `gofmt`, `go vet -tags unit,integration,e2e ./...`,
`make lint-go`, unit tests, and the integration or e2e tests the change
touches.

## Code conventions

- Contracts first: change `apispecs/`, run `make generate`, then the code.
- Services take domain objects for data and IDs for lookups, return domain
  objects, and never generate IDs (the handler does). Services use
  `time.Now()`; no injected clocks.
- Expected conditions are domain sentinel errors (`ErrNotFound`,
  `ErrAlreadyExists`, `ErrOtherGateway`), returned directly, checked with
  `errors.Is`. Handlers map only contract errors to gRPC codes / HTTP
  status; anything else is returned as is, and the middleware logs it and
  answers INTERNAL / 500.
- Input validation lives in the handlers; services hold business rules.
- Optional fields are pointers. Repository update methods are named
  `Update<Entity><What>`.
- Fakes live next to their consumer: `repo_fake.go` in the service
  package, `service_fake.go` in `grpc/` and `handler/`. Fakes are simple:
  exported slices, an `Err` field, plain loops.
- Tests: external test packages (`<pkg>_test`), table-driven with an
  explicit `want`, `cmp.Diff`, one field per line. REST handler tests
  validate every response with the ogen `Validate()` methods.
- E2E tests are plain Go, not Cucumber: `step(t, "Given/When/Then …")`
  subtests, unique serial numbers per scenario.
- Follow *100 Go Mistakes*: no shadowing, documented exports.

## Working with the author

- Plan first, then build in small, focused steps; each commit contains only
  the agreed step.
- Commit messages are numbered: `commit N: <what>` (see `git log`), with a
  short body. Group related commits into one PR.
- **No Claude attribution** in commits or PRs: no `Co-Authored-By`, no
  "Generated with Claude Code".
- Don't start Docker builds or long runs, commit, or push without being
  asked. Ask before anything outside the repo (repository visibility,
  invites, stopping other containers).
- Earlier PRs are merged quickly; before pushing, check whether the PR is
  still open (`gh pr view <n>`) and open a new one if not.
