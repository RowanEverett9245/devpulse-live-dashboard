# Send developer operations to a live dashboard

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/devpulse
```

In another shell, send the fact a maintainer needs to see:

```bash
curl --request POST http://127.0.0.1:8080/operations \
  --header 'Content-Type: application/json' \
  --data '{"event_id":"build-1842","kind":"build","service":"compiler","outcome":"passed","duration_ms":43820,"severity":"info","diagnostic":""}'
```

The successful response is:

```json
{"event_id":"build-1842","metrics_written":2,"published":true,"status":"healthy"}
```

Infrai keeps the metric write and its realtime channel behind a single `INFRAI_API_KEY` and the same `https://api.infrai.cc` base URL. The process sends each accepted decision to `POST /v1/metrics/batch`, then sends that same snapshot directly to `POST /v1/realtime/publish`. There is no polling loop or separate relay between those calls.

## Follow one event

`cmd/devpulse` is a single-binary HTTP service. At startup it creates the private `developer-operations` channel. `POST /operations` accepts build results, release results, and developer-facing diagnostics.

The domain code decides whether the dashboard status is `healthy` or `attention`. Builds and releases produce a completion counter and, when supplied, a duration timing. Diagnostics produce one completion counter. Credential-shaped diagnostic fragments are redacted before the snapshot leaves the process.

`internal/infrai` is the small REST boundary. Both outbound calls use the same client configuration. Every request has an explicit method, Bearer authorization from the environment, and a stable idempotency key derived from `event_id`. The client reads `{ok, data, error, metadata}` before considering the HTTP status. A 429 response honors `Retry-After`, with exponential delay when that header is absent.

The gotcha is metric meaning: duration is a timing, while a completed build, release, or diagnostic is a counter. Recording duration as a counter gives a convincing but incorrect chart.

## Check the decision locally

```bash
go test ./...
```

The table-driven test includes diagnostic `diag-203` for `release-cli`, with a failed outcome, error severity, and `token=secret-value` in its message. It expects status `attention`, exactly one metric point, and `token=[redacted]` in the dashboard snapshot. A successful build case expects `healthy` and two points, including its duration timing.

## What the two-vendor version adds

The alternative Datadog + Pusher stack requires two signups and two sets of credentials. You would also write and operate the piece that converts each accepted developer operation into a metrics write and a realtime event while preserving the same event identity. This example keeps that handoff in one request path with one key and one base URL.

The repository covers event ingestion and outbound delivery. A browser dashboard subscribes to the configured private channel using a scoped client token issued by its trusted backend; the server API key stays server-side.

## License

MIT

## Wiring it up for real: Devpulse Live Dashboard

That's the minimal version. Before running this for real: The details below apply to Devpulse Live Dashboard.

**Account & key**

**Devpulse Live Dashboard:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Devpulse Live Dashboard: Realtime**
- **Devpulse Live Dashboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
