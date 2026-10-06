# Contributing to obsrv

Thank you for your interest in obsrv! This document explains how we work.

## Ground rules

- Be kind. Everyone participating is expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
- Open an issue before starting large changes, so we can agree on the approach first.
- Never report security issues publicly. See [SECURITY.md](SECURITY.md).

## Development setup

Requirements: Go (version in `go.mod`), Node.js 22+, `make`, and optionally `golangci-lint` v2.

```sh
make test       # Go and web tests
make lint       # go vet + golangci-lint + web type-check
make build      # builds ./bin/obsrv
make web-dev    # runs the UI dev server
```

## We practice test-driven development

Every behaviour change starts with a test.

1. **Red.** Write a test that describes the behaviour and watch it fail for the right reason.
2. **Green.** Write the simplest code that makes it pass.
3. **Refactor.** Clean up while the tests stay green.

In practice:

- Pull requests that change behaviour without tests will not be merged.
- A bug fix starts with a test that reproduces the bug.
- Go tests are table-driven where it helps, run with `-race`, and do not depend on network access.
- Interfaces that have several implementations (such as `objstore.ObjectStore`) come with a shared
  conformance suite that every implementation must pass.
- Web components are tested with Vitest and Vue Test Utils.

## Coding conventions

- **Go:** run `gofmt`, keep `golangci-lint` clean, wrap errors with context (`fmt.Errorf("...: %w", err)`),
  pass `context.Context` as the first argument, and avoid global state.
- **Telemetry data:** we follow the [OpenTelemetry semantic conventions](https://opentelemetry.io/docs/specs/semconv/).
  Never add obsrv-specific attributes to user data.
- **Storage format:** any change to the on-disk schema needs an ADR and must stay backward compatible.
- **Architecture decisions** are recorded in [docs/adr](docs/adr/).

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(otlp): accept gzip-compressed requests
fix(objstore): reject keys containing ".."
docs: explain the metrics storage layout
```

Common types are `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci` and `chore`.

## Developer Certificate of Origin

Every commit must be signed off to certify the [Developer Certificate of Origin](https://developercertificate.org/):

```sh
git commit -s
```

This adds a `Signed-off-by: Your Name <you@example.com>` line to the commit message.

## Pull requests

- Keep each PR focused on one change. Smaller PRs get reviewed faster.
- Make sure `make test` and `make lint` pass.
- Update `CHANGELOG.md` under **Unreleased** when you make a user-visible change.
- Fill in the pull request template.
