# Builds both binaries once; each target below is a small image for one of
# them. Select one with --target (docker-compose.yml does).

FROM golang:1.26 AS build
WORKDIR /src
# Dependencies first, so they are cached until go.mod or go.sum change.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/fleet ./cmd/fleet \
 && CGO_ENABLED=0 go build -o /out/simulator ./cmd/simulator

# The server: REST on 8080, gRPC on 9090.
FROM gcr.io/distroless/static-debian12:nonroot AS fleet
COPY --from=build /out/fleet /fleet
EXPOSE 8080 9090
ENTRYPOINT ["/fleet"]

# The gateway simulator.
FROM gcr.io/distroless/static-debian12:nonroot AS simulator
COPY --from=build /out/simulator /simulator
ENTRYPOINT ["/simulator"]
