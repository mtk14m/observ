# syntax=docker/dockerfile:1
FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY *.go ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/demo-shop .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/demo-shop /demo-shop
ENTRYPOINT ["/demo-shop"]
