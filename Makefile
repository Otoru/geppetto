GO ?= go
GO_BIN := $(if $(shell $(GO) env GOBIN),$(shell $(GO) env GOBIN),$(shell $(GO) env GOPATH)/bin)

# Tool resolution. An explicit override (make BUF=/path/to/buf) always wins.
# Otherwise the tool is searched in GOBIN/GOPATH-bin first (where
# `go install` places version-pinned tools) and then on the PATH (where CI
# setup actions such as bufbuild/buf-setup-action place them). A target must
# never pass because a tool was missing or unable to operate this module:
# resolution failures and version mismatches fail loudly with install
# instructions.
find-tool = $(firstword $(wildcard $(GO_BIN)/$(1)) $(shell command -v $(1) 2>/dev/null))

BUF ?= $(call find-tool,buf)
AIR ?= $(call find-tool,air)
GOLANGCI_LINT ?= $(call find-tool,golangci-lint)

# The CI runs golangci-lint v2.14.0. Older binaries can panic on this
# module's Go version or silently report nothing, so lint requires at least
# the CI version and fails loudly instead of producing a misleading pass.
GOLANGCI_LINT_MIN := 2.14.0

# Fallback version stamp for builds outside a git checkout; mirrors the
# in-tree default of cmd/geppetto (var Version = developmentVersion).
DEV_VERSION := dev
VERSION := $(shell git describe --always --dirty 2>/dev/null || echo $(DEV_VERSION))

# Cross-compile matrix as GOOS/GOARCH pairs; the windows entry gains .exe.
PLATFORMS := linux/amd64 darwin/arm64 windows/amd64

.PHONY: generate lint test bench build build-all dev

# Development hot reload via air (github.com/air-verse/air).
# Dev tool only: not a build or CI dependency.
dev:
	@test -n "$(AIR)" || { echo "air not found; install with: $(GO) install github.com/air-verse/air@latest (or pass AIR=/path/to/air)" >&2; exit 1; }
	$(AIR)

generate:
	@test -n "$(BUF)" || { echo "buf not found; install with: $(GO) install github.com/bufbuild/buf/cmd/buf@latest (or pass BUF=/path/to/buf)" >&2; exit 1; }
	$(BUF) generate

lint:
	@test -n "$(BUF)" || { echo "buf not found; install with: $(GO) install github.com/bufbuild/buf/cmd/buf@latest (or pass BUF=/path/to/buf)" >&2; exit 1; }
	@test -n "$(GOLANGCI_LINT)" || { echo "golangci-lint not found; install with: $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_MIN) (or pass GOLANGCI_LINT=/path/to/golangci-lint)" >&2; exit 1; }
	@version=$$($(GOLANGCI_LINT) version 2>/dev/null | sed -n 's/.*has version \([0-9][0-9.]*\).*/\1/p'); \
	if [ -z "$$version" ]; then \
		echo "cannot read a version from $(GOLANGCI_LINT); need golangci-lint >= $(GOLANGCI_LINT_MIN)" >&2; exit 1; \
	fi; \
	if ! printf '%s %s\n' "$(GOLANGCI_LINT_MIN)" "$$version" | awk '{ split($$1, m, "."); split($$2, v, "."); for (i = 1; i <= 3; i++) { if (v[i]+0 > m[i]+0) exit 0; if (v[i]+0 < m[i]+0) exit 1 } }'; then \
		echo "golangci-lint $$version at $(GOLANGCI_LINT) is older than $(GOLANGCI_LINT_MIN), the CI version; a stale linter can miss findings this module fails on. Install with: $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$(GOLANGCI_LINT_MIN)" >&2; exit 1; \
	fi
	$(BUF) lint
	$(GOLANGCI_LINT) run ./...

test:
	$(GO) test ./...

bench:
	$(GO) test -bench=. -benchmem . ./internal/service

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags "-X main.Version=$(VERSION)" -o bin/geppetto ./cmd/geppetto

build-all:
	mkdir -p bin
	$(foreach platform,$(PLATFORMS),GOOS=$(word 1,$(subst /, ,$(platform))) GOARCH=$(word 2,$(subst /, ,$(platform))) $(GO) build -trimpath -ldflags "-X main.Version=$(VERSION)" -o bin/geppetto-$(subst /,-,$(platform))$(if $(filter windows/%,$(platform)),.exe) ./cmd/geppetto;)
