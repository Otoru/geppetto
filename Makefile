GO ?= go
GO_BIN := $(if $(shell $(GO) env GOBIN),$(shell $(GO) env GOBIN),$(shell $(GO) env GOPATH)/bin)
BUF ?= $(GO_BIN)/buf

.PHONY: generate lint test bench build build-all

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
	$(GO) build -trimpath -ldflags "-X main.Version=$$(git describe --always --dirty 2>/dev/null || echo dev)" -o bin/npcai ./cmd/npcai

build-all:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -o bin/npcai-linux-amd64 ./cmd/npcai
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -o bin/npcai-darwin-arm64 ./cmd/npcai
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -o bin/npcai-windows-amd64.exe ./cmd/npcai
