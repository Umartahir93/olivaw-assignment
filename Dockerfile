# Builds both binaries once; each target at the end is a small image for one
# of them. Select one with --target (docker-compose.yml does).

# Dependencies come from vendor/: nothing is downloaded during the build.
FROM golang:1.26 AS base
WORKDIR /src
# Static binaries; also part of the build cache key, so it must not change
# between the stages below.
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
COPY vendor/ vendor/

# The third-party packages the binaries import. Listing them is fast; the
# list only changes when an import or a dependency version does.
FROM base AS deps
COPY . .
RUN go list -deps -f '{{if not .Standard}}{{if not .Module.Main}}{{.ImportPath}}{{end}}{{end}}' ./cmd/... > /deps.txt

FROM base AS build
# Compile the dependencies in their own layer. Docker reuses it as long as
# the list is the same, so a code change only compiles this module's
# packages. -v prints every package, so a long build shows its progress.
COPY --from=deps /deps.txt /deps.txt
RUN xargs go build -v < /deps.txt
COPY . .
RUN go build -v -o /out/fleet ./cmd/fleet \
 && go build -v -o /out/simulator ./cmd/simulator

# The server: REST on 8080, gRPC on 9090.
FROM gcr.io/distroless/static-debian12:nonroot AS fleet
COPY --from=build /out/fleet /fleet
EXPOSE 8080 9090
ENTRYPOINT ["/fleet"]

# The gateway simulator.
FROM gcr.io/distroless/static-debian12:nonroot AS simulator
COPY --from=build /out/simulator /simulator
ENTRYPOINT ["/simulator"]
