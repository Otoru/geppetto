GO ?= go
GO_BIN := $(if $(shell $(GO) env GOBIN),$(shell $(GO) env GOBIN),$(shell $(GO) env GOPATH)/bin)
BUF ?= $(GO_BIN)/buf
AIR ?= $(GO_BIN)/air

# Fallback version stamp for builds outside a git checkout; mirrors the
# in-tree default of cmd/geppetto (var Version = developmentVersion).
DEV_VERSION := dev
VERSION := $(shell git describe --always --dirty 2>/dev/null || echo $(DEV_VERSION))

# Cross-compile matrix as GOOS/GOARCH pairs; the windows entry gains .exe.
PLATFORMS := linux/amd64 darwin/arm64 windows/amd64

.PHONY: generate lint test bench build build-all dev

# Hot reload de desenvolvimento via air (github.com/air-verse/air).
# Ferramenta de dev apenas: nao e dependencia de build nem de CI.
dev:
	@command -v $(AIR) >/dev/null 2>&1 || { echo "air nao encontrado; instale com: $(GO) install github.com/air-verse/air@latest"; exit 1; }
	$(AIR)

generate:
	$(BUF) generate

lint:
	$(BUF) lint
	golangci-lint run ./...

test:
	$(GO) test ./...

bench:
	$(GO) test -bench=. -benchmem ./internal/engine ./internal/server

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags "-X main.Version=$(VERSION)" -o bin/geppetto ./cmd/geppetto

build-all:
	mkdir -p bin
	$(foreach platform,$(PLATFORMS),GOOS=$(word 1,$(subst /, ,$(platform))) GOARCH=$(word 2,$(subst /, ,$(platform))) $(GO) build -trimpath -o bin/geppetto-$(subst /,-,$(platform))$(if $(filter windows/%,$(platform)),.exe) ./cmd/geppetto;)
