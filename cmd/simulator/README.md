# Gateway simulator

A **fake**: it simulates one virtual gateway that sends its robots'
measurements to the fleet server. It exists so the server side can be run
and tested with realistic traffic, without real robots or gateways.

It was written with the least effort, as part of the assignment. The server
side is the focus; the simulator is not a reference implementation of the
gateway contract.

## What it does

1. **Starts as a registered gateway.** Registering a gateway is an
   operator's job over the REST API, done before the gateway is installed.
   The simulator gets the resulting gatewayId with `-gateway-id` and sends
   it with every gRPC call (`x-gateway-id`).
2. **Scans its robots.** It asks the server which robots it already knows
   (`ListRobots`), then reports its full robot list (`ReportScan`) and gets
   a robotId for every robot. Robot serial numbers are derived from the
   gatewayId, so a restarted simulator reports the same robots.
3. **Keeps sending.** Every robot sends one measurement per interval
   (`ReportMeasurement`): battery, position, activity. The robots drive
   around a small map, use up their battery and go back to a charger
   (`pkg/simulator`).

Once started, it keeps sending until it is stopped (Ctrl-C or SIGTERM):

- If the server is not reachable at startup, it retries the scan with a
  growing pause (1 s up to 30 s) until the server answers.
- A failed measurement is logged and dropped; the robot sends the next one
  at the next interval.
- It only stops early on errors a retry cannot fix: a malformed gatewayId
  (`UNAUTHENTICATED`) or a rejected robot list (`INVALID_ARGUMENT`).

## Run it

```sh
# Database and server in Docker.
make up

# The operator registers the gateway; copy the gatewayId it returns.
curl -X POST localhost:8080/v1/gateways \
  -H 'Content-Type: application/json' \
  -d '{"serialNumber": "GW-001", "location": "Munich HQ"}'

# The simulator in Docker, and its logs.
make simulate GATEWAY_ID=<gatewayId>
make logs
```

Or locally, against a server on `localhost:9090`:

```sh
make run-simulator GATEWAY_ID=<gatewayId>
```

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `-gateway-id` | (required) | gatewayId from the gateway's registration |
| `-server` | `localhost:9090` | gRPC server address |
| `-robots` | `10` | number of simulated robots |
| `-interval` | `1s` | time between two measurements of one robot |
| `-log-level` | `info` | `debug` also logs every measurement |

## What it leaves out

- **Faults:** it never calls `ReportFault`, and measurements carry no
  active fault codes.
- **Delivery rules** (`docs/decisions.md`, 11): no buffer, no retries with
  backoff and no dead-letter queue for measurements; a failed one is
  dropped.
- **Rescans:** it scans once at startup. Robots never join, leave or go
  silent, so none of them ever turns offline.
- **More than one gateway per process:** run more simulators for more
  gateways, each with its own registration.
