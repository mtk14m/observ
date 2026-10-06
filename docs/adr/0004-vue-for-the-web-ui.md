# 0004. Vue 3 for the web UI

- Status: Accepted
- Date: 2026-10-06

## Context

The UI is embedded in the obsrv binary and must stay small, fast, and pleasant to work on.

## Decision

The UI uses Vue 3 (Composition API, `<script setup>`), TypeScript, Vite, Vue Router, Vitest and
Vue Test Utils. Time-series charts use uPlot. Fonts and assets are bundled so the UI never calls a CDN.

## Consequences

- We get a small bundle and single-file components that are easy to read.
- Vue's ecosystem of observability components is smaller than React's, so we build our own components.
- The built assets (`web/dist`) are embedded in the Go binary with `go:embed`.
