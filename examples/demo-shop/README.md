# demo-shop

A small shop made of four services, used to demo obsrv. It is instrumented only with the official
OpenTelemetry Go SDK and configured with the standard `OTEL_*` environment variables, so it works
with any OpenTelemetry backend.

| Role | What it does |
|---|---|
| `frontend` | Serves `/products` and `/checkout`, and generates traffic on itself |
| `checkout` | Places orders: reserves stock, then charges the payment |
| `inventory` | Serves stock levels and reservations, and reports a stock gauge |
| `payment` | Charges cards. One issuer, `acme-bank`, is often slow and refuses about a quarter of its cards |

The easiest way to run it is the Docker Compose demo at the root of the repository (`make demo`).
To run one service by hand:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 go run . -role payment -addr :8004
```
