# Tiny Analytics

Privacy-friendly real-time analytics pipeline built with Go, Redpanda, and ClickHouse. It ships with ingest + query APIs, Kafka-based enrichment, a ClickHouse loader, and observability via Prometheus/Grafana.

```
┌─────────────┐     events.raw      ┌──────────────┐     events.enriched     ┌────────────┐
│  Ingest API │ ───────────────────▶│ UA/IP Enrich │ ───────────────────────▶ │  Loader    │
└─────┬───────┘                     └──────┬───────┘                          └─────┬──────┘
      │                                   │                                       │
      │  REST HMAC+JSON                   │ Prom metrics                           │ ClickHouse
      ▼                                   ▼                                       ▼
 Public clients                    Prometheus + Grafana dashboards        Query API (pageviews, uniques, top pages)
```

## Components

- **Ingest API** (`cmd/ingest-api`): validates/HMAC-checks events, adds IP/UA metadata, enqueues to `events.raw`.
- **Consumer Enricher** (`cmd/consumer-enricher`): parses UTM/device/IP hash, emits enriched JSON to `events.enriched`.
- **Loader** (`cmd/loader`): batches Kafka messages and inserts into ClickHouse with retries + histograms.
- **Query API** (`cmd/query-api`): exposes `/v1/metrics/{pageviews,unique-users,top-pages}` with cached responses.
- **Infra** (`deployments/`): Redpanda, ClickHouse, Prometheus, Grafana (with datasource + starter dashboard).
- **Observability**: every service exposes `/metrics`; Prom/Grafana prewired.

## Quickstart

1. **Bootstrap env & deps**
   ```bash
   cp .env.example .env
   make tidy
   make infra-up
   ```
   Update `config/sites.dev.yml` (or point `SITES_CONFIG_PATH` elsewhere) with the site IDs + API keys you want to allow. The repo ships with `site-1`/`site-2` dev credentials so you can run immediately.
2. **Run the Go services (separate terminals)**
   ```bash
   source .env
   make run-ingest
   make run-consumer
   make run-loader
   make run-query
   ```
3. **Seed demo data & explore**
   ```bash
   ./scripts/seed.sh
   curl "http://localhost:8081/v1/metrics/pageviews?site_id=site-1&from=2024-01-01&to=2024-12-31"
   ```
4. **Grafana / Prometheus**
   - Grafana: http://localhost:3000 (admin/admin) → “Tiny Analytics Overview” dashboard.
   - Prometheus: http://localhost:9090
5. **Shut everything down**
   ```bash
   make infra-down
   ```

### Sample curl commands

```bash
# Track an event (replace HMAC header if HMAC_SECRET set)
curl -X POST http://localhost:8080/v1/collect \
  -H "Content-Type: application/json" \
  -H "X-TA-API-Key: dev-site-1-key" \
  -d '{
        "site_id": "site-1",
        "event_name": "pageview",
        "url": "https://example.com/docs",
        "referrer": "https://search.example",
        "user_id": "user-123",
        "session_id": "sess-456",
        "ts": 1700000000000,
        "props": {"plan": "pro"}
      }'

# Query metrics
curl "http://localhost:8081/v1/metrics/top-pages?site_id=site-1&from=2024-01-01&to=2024-12-31&limit=5"
```

## Security & Privacy Choices

- **Per-site API keys**: `config/sites.dev.yml` (configurable via `SITES_CONFIG_PATH`) binds each `site_id` to an API key and optional per-tenant HMAC secret; `/v1/collect` rejects events without the correct `X-TA-API-Key`.
- **HMAC ingestion**: optional `HMAC_SECRET` enforces `X-TA-Signature` using SHA-256 for tamper detection.
- **IP hashing**: raw IP never stored; salted SHA-256 (`IP_HASH_SALT`) retained only for uniqueness analytics.
- **Bot filtering**: configurable UA deny list drops obvious bots before they enter the pipeline.
- **CORS allow-list**: `CORS_ALLOW_ORIGINS` keeps ingest locked down in production, `*` for local dev by default.
- **PII minimization**: payload stored as opaque JSON (`payload` column); no server-side enrichment adds new PII.

## Testing

```bash
make test        # unit tests
make test-e2e    # spins docker infra + runs end-to-end flow (requires docker + go)
```

## Roadmap

- [x] Kafka ingest → enrich → ClickHouse load pipeline
- [x] Prometheus metrics + starter Grafana dashboard
- [x] Seed script and integration test harness
- [x] Multi-tenant auth / API keys per site
- [ ] UI for self-service dashboards

## Troubleshooting

- Ensure Docker Desktop (or engine) has at least 4GB RAM for ClickHouse.
- If Prometheus can’t scrape local services on Linux, verify Docker supports `host.docker.internal` (Docker ≥ 20.10) or update `deployments/prometheus/prometheus.yml` with your host IP.
