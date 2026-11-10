BIN := $(CURDIR)/bin
export PATH := $(BIN):$(PATH)

##@ General

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*## "} /^##@/ {printf "\n%s\n", substr($$0, 5)} /^[a-z0-9_-]+:.*## / {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: generate
generate: grpc-generate rest-generate ## Generate Go code from both API contracts into ./gen.

##@ Tools

BUF_VERSION                := v1.75.0
PROTOC_GEN_GO_VERSION      := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2
GOLANGCI_LINT_VERSION      := v2.15.0
OGEN_VERSION               := v1.24.0

BUF                := $(BIN)/buf
PROTOC_GEN_GO      := $(BIN)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(BIN)/protoc-gen-go-grpc
GOLANGCI_LINT      := $(BIN)/golangci-lint
OGEN               := $(BIN)/ogen
TOOLS := $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) $(GOLANGCI_LINT) $(OGEN)

# Each tool is a file target, so any target that needs one installs it on
# first use. After changing a version, run `make clean-tools tools`.
$(BUF):
	GOBIN=$(BIN) go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
$(PROTOC_GEN_GO):
	GOBIN=$(BIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
$(PROTOC_GEN_GO_GRPC):
	GOBIN=$(BIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
$(GOLANGCI_LINT):
	GOBIN=$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
$(OGEN):
	GOBIN=$(BIN) go install github.com/ogen-go/ogen/cmd/ogen@$(OGEN_VERSION)

.PHONY: tools
tools: $(TOOLS) ## Install all pinned tools into ./bin.

.PHONY: clean-tools
clean-tools: ## Remove the tools installed in ./bin.
	rm -rf $(BIN)

##@ Go

# Each kind of test has its own build tag and runs on its own:
#   unit         //go:build unit         no external dependencies
#   integration  //go:build integration  real dependencies, e.g. the database
#   e2e          //go:build e2e          the running system, called over its APIs
.PHONY: test_unit
test_unit: ## Run the unit tests (files tagged //go:build unit).
	go test -tags unit -race ./...

# Integration tests share one test database and empty its tables before
# every case, so packages run one after another (-p 1).
.PHONY: test_integration
integration_test: db-up ## Run the integration tests against the test database (files tagged //go:build integration).
	go test -tags integration -race -count=1 -p 1 ./...

.PHONY: test_e2e
test_e2e: ## Run the end-to-end tests (files tagged //go:build e2e).
	go test -tags e2e -count=1 ./...

.PHONY: lint-go
lint-go: $(GOLANGCI_LINT) ## Lint the Go code with golangci-lint.
	$(GOLANGCI_LINT) run --build-tags unit,integration,e2e ./...

##@ Docker

# Compose is a docker plugin on some machines and a standalone binary on others.
COMPOSE  := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo docker-compose)
GO_IMAGE := golang:1.26

# Run Go tools in containers with the source mounted, sharing caches between runs.
DOCKER_GO := docker run --rm -v $(CURDIR):/src -w /src -v olivaw-assignment_go-mod:/go/pkg/mod -v olivaw-assignment_go-build:/root/.cache/go-build

.PHONY: up
up: ## Build and start everything in Docker: database, server (REST :8080, gRPC :9090) and simulator.
	$(COMPOSE) up -d --build

.PHONY: down
down: ## Stop everything started with up or db-up.
	$(COMPOSE) down

.PHONY: logs
logs: ## Follow the logs of the server and the simulator.
	$(COMPOSE) logs -f fleet simulator

.PHONY: db-up
db-up: ## Start only the database (fleet and fleet_test) and wait until it is ready.
	$(COMPOSE) up -d --wait db

.PHONY: test_unit_docker
test_unit_docker: ## Run the unit tests in a Go container.
	$(DOCKER_GO) $(GO_IMAGE) go test -tags unit -race ./...

.PHONY: test_integration_docker
test_integration_docker: ## Run the integration tests in a Go container next to the database.
	$(COMPOSE) --profile test run --rm integration-tests

.PHONY: docker_test
docker_test: test_integration_docker test_unit_docker ## Run unit and integration tests in Docker.

.PHONY: docker_lint
docker_lint: ## Lint the Go code in a golangci-lint container.
	$(DOCKER_GO) golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) golangci-lint run --build-tags unit,integration,e2e ./...

##@ Simulator

.PHONY: run-simulator
run-simulator: ## Run simulated robots against the gRPC server on localhost:9090.
	go run ./cmd/simulator

##@ gRPC (apispecs/grpc)

GRPC_DIR := apispecs/grpc

# Branch the contract is checked against for breaking changes.
BREAKING_AGAINST ?= .git\#branch=main,subdir=$(GRPC_DIR)

.PHONY: grpc-lint
grpc-lint: $(BUF) ## Lint the proto contract.
	$(BUF) lint $(GRPC_DIR)

.PHONY: grpc-format
grpc-format: $(BUF) ## Format the proto files in place.
	$(BUF) format -w $(GRPC_DIR)

.PHONY: grpc-format-check
grpc-format-check: $(BUF) ## Fail if the proto files are not formatted.
	$(BUF) format --diff --exit-code $(GRPC_DIR)

.PHONY: grpc-breaking
grpc-breaking: $(BUF) ## Fail if the contract breaks clients built against BREAKING_AGAINST.
	$(BUF) breaking $(GRPC_DIR) --against '$(BREAKING_AGAINST)'

.PHONY: grpc-check
grpc-check: grpc-lint grpc-format-check ## Lint and format check (run before every commit).

.PHONY: grpc-generate
grpc-generate: $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) ## Generate the gRPC code into ./gen/gateway.
	rm -rf gen/gateway
	$(BUF) generate $(GRPC_DIR) --template $(GRPC_DIR)/buf.gen.yaml

##@ REST (apispecs/rest)

REST_DIR := apispecs/rest

.PHONY: rest-generate
rest-generate: $(OGEN) ## Generate the REST server and models into ./gen/fleet/v1.
	$(OGEN) --config $(REST_DIR)/ogen.yaml --target gen/fleet/v1 --package fleetv1 --clean $(REST_DIR)/openapi.yaml
