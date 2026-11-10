# Decisions

The main design decisions, why they were made, and what they cost. Each
one is a trade-off; "Revisit when" says what would change it.

## 1. gRPC for robots, REST for people

**Decision.** Robots send telemetry over gRPC. Admins and dashboards use a
REST API.

**Why.** The two clients are different. Robots send many small messages
from devices: gRPC gives a compact binary format, a typed contract and
generated clients. People use browsers and scripts: browsers cannot call
gRPC without a proxy, while REST with JSON works with `curl`, any frontend
and HTTP caching.

**Cost.** Two contracts and two toolchains (buf, ogen).

## 2. API-first, with generated code

**Decision.** Both contracts live in `apispecs/` and are written before the
code. Go code is generated into `gen/`; handlers are written by hand.

- gRPC: buf lints the proto (naming plus required doc comments), checks
  formatting, and fails on breaking changes against `main`.
- REST: ogen generates models, validation, routing and the server. Handlers
  in `api/` implement the generated `fleetv1.Handler` interface.

**Why.** The contract is the source of truth. If the spec gains an
endpoint, the build fails until a handler implements it, so code and spec
cannot drift. Parameters and bodies are validated from the spec, not by
hand.

**Cost.** Routing is ogen's, not `net/http`'s; moving away from ogen would
mean rewriting the server wiring (the handlers would barely change).
Rules OpenAPI cannot express, like "at most 24 hours between `from` and
`to`", still need code.

## 3. Unary RPCs, one item per request

**Decision.** `ReportTelemetry` and `ReportFault` are unary calls, each
carrying exactly one reading or fault.

**Why.**
- At about 1 message per second per robot, a stream saves almost nothing:
  the HTTP/2 connection stays open either way.
- The OK response is the acknowledgement. The robot can delete the reading
  from its buffer once it gets OK.
- Unary calls are load-balanced per request. A long-lived stream sticks to
  one server instance, which matters on Kubernetes.
- One item per request means no partial failures: a request is either
  stored or rejected as a whole.

**Cost.** After a long outage the robot sends its backlog one call per
reading (one hour is 3,600 calls).

**Revisit when** backlogs get large: add a batch method next to the
existing one, without breaking it.

## 4. Separate methods per intent

**Decision.** State snapshots (`ReportTelemetry`) and faults
(`ReportFault`) are separate methods. Each snapshot also carries the codes
of the currently active faults.

**Why.** A snapshot answers "what is the robot's state now"; losing one
does not matter because the next one replaces it. A fault is something
that happened once, with its own details. Listing active codes in every
snapshot means a fault clears itself when its code disappears, without
needing a "fault cleared" event.

**Cost.** Faults are matched by code only, so two faults with the same
code merge. A code can show up in a snapshot before its details arrive.

**Revisit when** fault lifecycles matter (who fixed it, how long it took):
add fault IDs and raised/cleared events.

## 5. Robot identity in metadata

**Decision.** The robot sends its ID in the `x-robot-id` gRPC metadata, not
in the message. The robot does not send its site or location.

**Why.** Identity belongs to the transport. In production it would come
from the robot's mTLS client certificate, so a robot could not claim to be
another one; only the code that reads the metadata would change. Where a
robot is deployed is decided by people at onboarding, not by the robot.

## 6. Onboarding: serial number in, UUID out

**Decision.** An admin registers a robot with its serial number and
location (`POST /v1/robots`). The server assigns a UUID as `robotId`, which
is configured on the robot. Decommissioning is a soft delete.

**Why.**
- The server owns the ID, so it is unique and never changes, even if a
  serial number was entered wrongly.
- Unregistered and decommissioned robots can be rejected at ingestion.
- A soft delete keeps the robot's history.

**Cost.** One extra step: the UUID has to be put on the robot. Serial
numbers must be unique across the fleet, which assumes no two vendors
reuse one.

**Not modelled.** Sites and maps as their own resources, vendor and model,
certificate issuing. They would come with real multi-site, multi-vendor
fleets.

## 7. Timestamps and deduplication without sequence numbers

**Decision.** Each reading carries `recorded_at` (robot clock). The server
stores its own receive time separately. Readings are deduplicated on
`(robot_id, recorded_at)`, faults on `(robot_id, code, occurred_at)`.
Timestamps are stored to the microsecond. The first write wins.

**Why.**
- `recorded_at` keeps buffered readings at the time they were taken, not
  the time they arrived.
- The server's receive time decides whether a robot is online, because
  the robot's clock can be wrong.
- Because retries can deliver a reading twice, a natural key makes
  resending safe without a sequence number.

**Cost.** This assumes robot clocks are roughly synced (NTP) and that a
robot takes at most one reading per microsecond. A robot booting with a
wrong clock produces readings with wrong times.

## 8. Delivery: buffer, retry, dead-letter queue

**Decision (in the contract; not yet in the simulator).** The robot buffers
readings while it cannot deliver them and sends them oldest first.

| Response | Robot does |
|---|---|
| OK | Delete the item from the buffer |
| `INVALID_ARGUMENT`, `UNAUTHENTICATED` | Drop it and log locally; resending fails the same way |
| Anything else | Retry up to 3 times with exponential backoff, then move it to a local dead-letter queue, log it and count it in a metric; an alert brings in an operator |

**Why.** History should have no holes, and one bad item must never block
the items behind it.

**Cost.** Even a short network outage moves items into the dead-letter
queue and needs a person.

**Revisit when** this gets noisy: alert on the queue's size or age instead
of each item, and replay items that failed only because the server was
unreachable automatically.

## 9. Server-derived connectivity

**Decision.** `online`, `stale`, `offline` and `never_seen` are computed by
the server from the last receive time, not reported by the robot.

**Why.** A robot that is offline cannot report that it is offline.

## 10. History: 24 hours per request, offset paging

**Decision.** Telemetry and fault history return at most 24 hours per
request, on any date. Lists use `offset` and `limit` (max 1000) and return
`hasMore`, not a total count.

**Why.** Simple to implement and to use. `hasMore` costs no extra query
(fetch one row more than asked). A total count would need a `COUNT(*)` on
a large time-series table for every page.

**Cost.** One robot-day at 1 Hz is up to 86,400 readings, so up to 87
pages. Rows that arrive while a client is paging shift later pages, so
clients must pass the same `from` and `to` for every page.

**Revisit when** dashboards need whole days: add an aggregated endpoint
(for example one value per minute) or cursor-based paging.

## 11. Errors as problem details

**Decision.** REST errors use RFC 9457 problem details
(`application/problem+json`). This includes the validation errors ogen
detects itself, through a custom error handler in `cmd/fleet`.

**Why.** One error format for every endpoint, based on a standard.

## 12. One binary for both servers

**Decision.** `cmd/fleet` runs the gRPC and the REST server in one process.

**Why.** Simplest to build, run and test while both are small.

**Revisit when** load differs: ingestion and the fleet API can become
separate binaries that scale independently. The servers are already
separate files in `cmd/fleet`, and the handlers live in their own
packages.

## 13. Storage (planned)

**Decision.** TimescaleDB, PostgreSQL with a time-series extension.

**Why.** Telemetry is append-only and time-ordered. TimescaleDB partitions
it by time, compresses older data into a columnar format and drops data
after a retention period, while staying PostgreSQL, which is already a
common choice for the rest of the data (robots, faults). ClickHouse was
considered; it is faster for fleet-wide analytics but would add a second
database to run, also on customer hardware.
