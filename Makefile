GO        ?= go
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS   := -s -w \
	-X github.com/mtk14n/obsrv/internal/version.Version=$(VERSION) \
	-X github.com/mtk14n/obsrv/internal/version.Commit=$(COMMIT)

.PHONY: all build test test-go test-web lint lint-go lint-web fmt web-install web-dev web-build clean

all: lint test build

build: ## Build the obsrv binary into ./bin
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/obsrv ./cmd/obsrv

test: test-go test-web ## Run all tests

test-go:
	$(GO) test -race -count=1 ./...

test-web: web-install
	cd web && npm test

lint: lint-go lint-web ## Run all linters

lint-go:
	$(GO) vet ./...
	golangci-lint run

lint-web: web-install
	cd web && npm run type-check

fmt:
	gofmt -s -w .

web-install:
	cd web && npm ci --no-audit --no-fund

web-dev:
	cd web && npm run dev

web-build: web-install
	cd web && npm run build

clean:
	rm -rf bin web/dist
