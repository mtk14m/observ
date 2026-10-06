# syntax=docker/dockerfile:1

# --- UI ----------------------------------------------------------------------
FROM node:24-trixie-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# --- Binary (cgo is required by the embedded DuckDB) --------------------------
FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /src/web/dist/ internal/ui/dist/
ARG VERSION=dev
ARG COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -trimpath \
      -ldflags "-s -w -X github.com/mtk14n/obsrv/internal/version.Version=${VERSION} -X github.com/mtk14n/obsrv/internal/version.Commit=${COMMIT}" \
      -o /out/obsrv ./cmd/obsrv

# --- Runtime -----------------------------------------------------------------
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --home /data obsrv \
    && mkdir -p /data && chown obsrv /data
COPY --from=build /out/obsrv /usr/local/bin/obsrv
USER obsrv
VOLUME /data
EXPOSE 4317 4318 8080
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["obsrv", "-data-dir", "/data"]
