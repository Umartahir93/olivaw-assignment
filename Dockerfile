# Builds both binaries once; each target at the end is a small image for one
# of them. Select one with --target (docker-compose.yml does).

# Dependencies come from vendor/: nothing is downloaded during the build.
FROM golang:1.26 AS base
WORKDIR /src
# Static binaries without local paths. Both settings are part of the build
# cache key, so every stage below must use the same ones.
ENV CGO_ENABLED=0 \
    GOFLAGS=-trimpath
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
# packages.
COPY --from=deps /deps.txt /deps.txt
RUN xargs go build < /deps.txt
COPY . .
# One build for both binaries, so they share the work. -s -w drops the
# debug information: the binaries are smaller and link faster.
RUN mkdir /out && go build -ldflags="-s -w" -o /out/ ./cmd/fleet ./cmd/simulator

# The server: REST on 8080, gRPC on 9090.
FROM gcr.io/distroless/static-debian12:nonroot AS fleet
COPY --from=build /out/fleet /fleet
EXPOSE 8080 9090
ENTRYPOINT ["/fleet"]

# The gateway simulator.
FROM gcr.io/distroless/static-debian12:nonroot AS simulator
COPY --from=build /out/simulator /simulator
ENTRYPOINT ["/simulator"]
