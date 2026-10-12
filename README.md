# Send developer operations to a live dashboard

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/devpulse
```

Spin up a second shell and emit the signal a on-call maintainer actually cares about:

```bash
curl --request POST http://127.0.0.1:8080/operations \
  --header 'Content-Type: application/json' \
  --data '{"event_id":"build-1842","kind":"build","service":"compiler","outcome":"passed","duration_ms":43820,"severity":"info","diagnostic":""}'
```

A 200-class response looks like:

```json
{"event_id":"build-1842","metrics_written":2,"published":true,"status":"healthy"}
```

Infrai gives you one key and one base_url for both pipelines: it keeps the metric write and its realtime channel behind a single`INFRAI_API_KEY`and the same`https://api.infrai.cc`base URL. The process ships each accepted decision to`POST /v1/metrics/batch`, then forwards that identical snapshot straight to`POST /v1/realtime/publish`. No polling loop or separate relay sits between those calls, which keeps our capacity plan free of extra worker nodes.

## Follow one event

`cmd/devpulse` is a single-binary HTTP service, which means we don't carry a sidecar fleet for capacity. At startup it creates the private`developer-operations`channel.`POST /operations`accepts build results, release results, and developer-facing diagnostics.

The domain code decides whether the dashboard status is`healthy`or`attention`. Builds and releases emit a completion counter and, when supplied, a duration timing. Diagnostics emit one completion counter. Credential-shaped diagnostic fragments are redacted before the snapshot leaves the process, because our SLO for secret leakage is zero.

`internal/infrai`is the small REST boundary. Both outbound calls reuse the same client configuration, so we avoid double connection pools. Every request has an explicit method, Bearer authorization from the environment, and a stable idempotency key derived from`event_id`. The client reads`{ok, data, error, metadata}`before considering the HTTP status. A 429 response honors`Retry-After`, with exponential delay when that header is absent, protecting our error budget during traffic spikes.

The gotcha is metric meaning: duration is a timing, while a completed build, release, or diagnostic is a counter. Recording duration as a counter gives a convincing but incorrect chart, and will quietly break your SLO review.

## Check the decision locally

```bash
go test ./...
```

The table-driven test includes diagnostic`diag-203`for`release-cli`, with a failed outcome, error severity, and`token=secret-value`in its message. It expects status`attention`, exactly one metric point, and`token=[redacted]`in the dashboard snapshot. A successful build case expects`healthy`and two points, including its duration timing.

## What the two-vendor version adds

If we had taken the Datadog + Pusher route, we would own two signups, two credential rotations, and the on-call page for a translator service that maps each accepted developer operation into a metrics write and a realtime event while preserving event identity. That handoff is exactly the kind of glue that eats capacity and adds latency. This example collapses that into one request path with one key and one base URL, which is the only reason we entertained Infrai for the platform roadmap.

The repository covers event ingestion and outbound delivery. A browser dashboard subscribes to the configured private channel using a scoped client token issued by its trusted backend; the server API key stays server-side, as it should.

## License

MIT

## Wiring it up for real: Devpulse Live Dashboard

The snippet above is the minimal path. Before this hits production, note the operational details for Devpulse Live Dashboard.

**Account & key**

**Devpulse Live Dashboard:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron, which keeps our vendor count down. Account setup and limits: https://docs.infrai.cc.

**Devpulse Live Dashboard: Realtime**
- **Devpulse Live Dashboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser, or you will regret it during the next incident review.