GO        ?= go
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS   := -s -w \
	-X github.com/mtk14n/obsrv/internal/version.Version=$(VERSION) \
	-X github.com/mtk14n/obsrv/internal/version.Commit=$(COMMIT)

.PHONY: all build build-go test test-go test-web lint lint-go lint-web fmt web-install web-dev web-build run demo demo-down clean

all: lint test build

build: web-build build-go ## Build the obsrv binary with the embedded UI into ./bin

build-go: ## Build the obsrv binary without rebuilding the UI
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/obsrv ./cmd/obsrv

run: build ## Build and run obsrv locally with data in ./data
	./bin/obsrv -data-dir ./data

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

web-build: web-install ## Build the UI and stage it for embedding
	cd web && npm run build
	find internal/ui/dist -mindepth 1 ! -name .keep -delete
	cp -R web/dist/. internal/ui/dist/

demo: ## Run obsrv with the demo shop in Docker, then open http://localhost:8080
	docker compose -f deploy/demo/docker-compose.yml up --build -d
	@echo "obsrv is starting: open http://localhost:8080 (data appears within ~15 seconds)"

demo-down: ## Stop the demo and delete its data
	docker compose -f deploy/demo/docker-compose.yml down -v

clean:
	rm -rf bin web/dist
	find internal/ui/dist -mindepth 1 ! -name .keep -delete
