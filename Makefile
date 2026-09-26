GO ?= go
GO_BIN := $(if $(shell $(GO) env GOBIN),$(shell $(GO) env GOBIN),$(shell $(GO) env GOPATH)/bin)
BUF ?= $(GO_BIN)/buf
AIR ?= $(GO_BIN)/air

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
	$(GO) build -trimpath -ldflags "-X main.Version=$$(git describe --always --dirty 2>/dev/null || echo dev)" -o bin/geppetto ./cmd/geppetto

build-all:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -o bin/geppetto-linux-amd64 ./cmd/geppetto
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -o bin/geppetto-darwin-arm64 ./cmd/geppetto
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -o bin/geppetto-windows-amd64.exe ./cmd/geppetto
