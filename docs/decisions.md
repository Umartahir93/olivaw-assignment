# Decisions

Each decision with why, what it costs, and when to revisit it. Terms are
defined in [architecture.md](architecture.md#terms).

## 1. Robots report through gateways

**Decision.** Robots never talk to the system; their gateway reports for
them. Here, the simulator is the gateway.

**Why.**
- The gateway translates many vendors' protocols into one contract.
- The gateway buffers data while the site is offline.
- One identity per site instead of one per robot.

**Cost.** Robots are known only through scans. A gateway that is down
silences all its robots.

## 2. gRPC for gateways, REST for people

**Why.**
- Gateways send many small messages: gRPC is compact, typed and generates
  clients.
- People use browsers and scripts: browsers cannot call gRPC without a
  proxy; REST with JSON works everywhere.

**Cost.** Two contracts, two toolchains (buf, ogen).

## 3. API-first, with generated code

**Decision.** Contracts in `apispecs/` come first. Code is generated into
`gen/`; handlers are written by hand.

- gRPC: buf lints, checks formatting and fails on breaking changes.
- REST: ogen generates models, validation, routing and the server; `api/`
  implements `fleetv1.Handler`.

**Why.**
- The contract is the source of truth. A new endpoint breaks the build
  until it is implemented. Input is validated from the spec.
- For the sake of simplicity, routing comes from ogen too, so there is no
  handwritten router and the project moves faster.

**Cost.** For the simplicity of this assignment, ogen's router is used
instead of a handwritten one. That ties the server to ogen: moving away
means rewriting the server wiring (handlers barely change). Rules OpenAPI
cannot express (the 24-hour range) still need code.

## 4. Two gRPC services, split by intent

**Decision.** `GatewayService`: `ListRobots`, `ReportScan`.
`RobotService`: `ReportMeasurement`, `ReportFault`.

**Why.** The robot list changes rarely; measurements arrive every second.
Separate services stay small and evolve independently.

## 5. Unary calls, one measurement per request

**Decision.** For now the simulator acts as a virtual gateway, and this is a
bare-minimum implementation: plain unary calls, one measurement per
request.

**Why.**
- At about 1 message per second per robot, streaming saves almost nothing.
- OK is the acknowledgement: the gateway can drop the item from its buffer.
- Unary calls load-balance per request; streams stick to one instance.
- No partial failures: a request is stored or rejected as a whole.
- `ReportScan` sends the full list, so repeating it changes nothing.

**Cost.** 100 robots means about 100 calls per second per gateway. A
backlog is sent one call per measurement.

## 6. Faults: one event, plus codes in every measurement

**Decision.** `ReportFault` once when a fault appears. Its code is listed
in every measurement while it is active.

**Why.** A fault clears itself when its code disappears; no "fault cleared"
event needed.

**Cost.** Faults with the same code merge. A code can appear before its
details arrive.

**Revisit when** fault lifecycles matter: add fault IDs and
raised/cleared events.

## 7. Gateway identity in metadata

**Decision.** `x-gateway-id` in the gRPC metadata. A gateway may only
report for its own robots.

**Why.** Identity belongs to the transport. In production it comes from
the gateway's mTLS certificate; only the code that reads it changes.

## 8. Serial number as business key, UUID as identity

**Decision.** An admin registers a gateway with serial number, location
and optional name. Robots are discovered by scans. The system gives each
a UUID that never changes. After a restart the gateway calls `ListRobots`
to keep its robots' IDs.

**Why.**
- Serial numbers are how the outside world recognises things.
- An ID the system owns is unique and stable.
- Robots need no admin.

**Cost.** The `gatewayId` must be configured on the gateway. Serial
numbers must be unique across the fleet. Robots never move between
gateways.

## 9. No decommissioning, no deleting

**Decision.** Gateways and robots stay once known. A robot missing from a
scan is kept.

**Why.** Simplicity.

**Cost.** Removed robots still show up, as `offline`.

**Revisit when** fleets change often.

## 10. Idempotency keys instead of sequence numbers

**Decision.** Measurements are keyed by `(robotId, measuredAt)`, faults by
`(robotId, code, occurredAt)`. A known key returns OK and is not stored
again. Timestamps are kept to the microsecond. The system stores its own
`receivedAt` separately.

**Why.**
- Resending is safe without sequence numbers.
- `measuredAt` keeps buffered data at its real time.
- `receivedAt` drives connectivity, because device clocks can be wrong.

**Cost.** Assumes clocks are roughly in sync (NTP).

## 11. Delivery: buffer, retry, dead-letter queue

| Response | Gateway does |
|---|---|
| OK | Drop the item from its buffer |
| `INVALID_ARGUMENT`, `UNAUTHENTICATED` | Drop it and log it |
| Anything else | Retry 3 times with backoff, then dead-letter queue, log, metric, alert |

**Why.** No holes in the history; a bad item never blocks the rest.

**Cost.** A short outage already needs a person.

**Revisit when** noisy: alert on queue size or age; auto-replay items that
failed only because the system was unreachable.

## 12. Connectivity derived by the system

**Decision.** `unknown` until first contact, `online` within 5 minutes,
then `offline`. For gateways and robots.

**Why.** Something offline cannot report it. Both levels show whether a
whole site or one robot is silent.

**Cost.** Up to 5 minutes to notice.

## 13. History: 24 hours per request, no pagination

**Decision.** At most 24 hours per request, any date. Lists return all
their items: pagination is not considered for now.

**Why.** Simple to build and to use.

**Cost.** One robot-day at 1 Hz is up to 86,400 measurements in one
response.

**Revisit when** lists or histories grow: pagination is worth adding, and
dashboards that need whole days would want aggregates (for example one
value per minute).

## 14. Errors as problem details

**Decision.** RFC 9457 for all REST errors, including ogen's own
validation errors.

**Why.** One standard format.

## 15. Direct calls, no pipeline

**Decision.** Each measurement is validated, stored and answered within
its gRPC call. No broker.

**Why.** Simple to build, run and test.

**Cost.** Ingestion waits for storage, spikes hit the database, and every
consumer lives in the same call.

**Revisit when** load grows: publish to a broker (Kafka, NATS JetStream)
and let consumers store, track and alert.

## 16. One binary for both servers

**Why.** Simplest while both are small.

**Revisit when** load differs: split ingestion and the fleet API into
separate binaries.

## 17. Storage: TimescaleDB (planned)

**Why.** Measurements are append-only and time-ordered. TimescaleDB
partitions by time, compresses old data and expires it, and is still
PostgreSQL for everything else.

**Not ClickHouse.** Faster for fleet-wide analytics, but a second database
to run, also on customer hardware.
