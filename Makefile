BIN := $(CURDIR)/bin
export PATH := $(BIN):$(PATH)
BUF := $(BIN)/buf

BUF_VERSION                := v1.75.0
PROTOC_GEN_GO_VERSION      := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2
GOLANGCI_LINT_VERSION      := v2.15.0

# Branch the contract is checked against for breaking changes.
BREAKING_AGAINST ?= .git\#branch=main

.PHONY: help
help: ## Show this help.
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-14s %s\n", $$1, $$2}'

PROTOC_GEN_GO      := $(BIN)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(BIN)/protoc-gen-go-grpc
GOLANGCI_LINT      := $(BIN)/golangci-lint
TOOLS := $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) $(GOLANGCI_LINT)

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

.PHONY: tools
tools: $(TOOLS) ## Install pinned buf, protoc plugins and golangci-lint into ./bin.

.PHONY: clean-tools
clean-tools: ## Remove the tools installed in ./bin.
	rm -rf $(BIN)

.PHONY: lint
lint: $(BUF) ## Lint the proto contract.
	$(BUF) lint

.PHONY: format
format: $(BUF) ## Format proto files in place.
	$(BUF) format -w

.PHONY: format-check
format-check: $(BUF) ## Fail if proto files are not formatted.
	$(BUF) format --diff --exit-code

.PHONY: breaking
breaking: $(BUF) ## Fail if the contract breaks clients built against BREAKING_AGAINST.
	$(BUF) breaking --against '$(BREAKING_AGAINST)'

.PHONY: generate
generate: $(TOOLS) ## Generate Go code from the proto contract into ./gen.
	$(BUF) generate

.PHONY: proto-check
proto-check: lint format-check ## Lint and format check (run before every commit).

.PHONY: test
test: ## Run the unit tests.
	go test -race ./...

.PHONY: lint-go
lint-go: $(GOLANGCI_LINT) ## Lint the Go code with golangci-lint.
	$(GOLANGCI_LINT) run ./...
