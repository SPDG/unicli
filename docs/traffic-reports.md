# Traffic reports

Live, read-only WAN and per-client traffic reports from the selected UniFi
controller. There is no local metrics database, collector, daemon, or
cached-history fallback.

Homelabward can answer “how much did this site download/upload since the start
of the month, and which client transferred the most data in that period?” with:

```bash
unicli network traffic wan --start 2026-09-01 --timezone Europe/Warsaw --json
unicli network traffic clients --start 2026-09-01 --timezone Europe/Warsaw --sort total --top 10 --json
```

Omit `--start` / `--end` for month-to-date through now. Date-only `--end` is
inclusive of that local calendar day. Maximum range is 32 calendar days.

These commands are read-only. `POST /stat/report/...` is a query, not a
configuration mutation, and does **not** require `--allow-mutations`.

## Schemas

- `unicli.network.traffic.wan/v1`
- `unicli.network.traffic.clients/v1`

Fixtures (synthetic identities only):

- `internal/network/testdata/traffic_wan_v1.example.json`
- `internal/network/testdata/traffic_clients_v1.example.json`

Discover flags with `unicli schema --json`.

## Endpoints

Integration v1 (`/proxy/network/integration/v1/...`) has no historical traffic
resource (404 `No endpoint GET /integration/v1/sites/{id}/traffic` on Network
10.6.101). The v2 Traffic Identification routes
`/proxy/network/v2/api/site/{site}/traffic` and `country-traffic` returned HTML
404 on the lab console (Traffic Identification not exposed).

unicli therefore uses the controller report API, same `X-API-KEY` and site slug
(`internalReference`, usually `default`) as other legacy-controller commands:

| Report | Method | Path |
|--------|--------|------|
| WAN site | POST | `/proxy/network/api/s/{site}/stat/report/{interval}.site` |
| Clients | POST | `/proxy/network/api/s/{site}/stat/report/{interval}.user` |

`{interval}` is `daily`, `hourly`, then `5minutes`. Body:

```json
{"attrs":["time","wan-rx_bytes","wan-tx_bytes","wan2-rx_bytes","wan2-tx_bytes"],"start":START_MS,"end":END_MS}
```

User reports request `time`, `rx_bytes`, `tx_bytes`, `wired-rx_bytes`,
`wired-tx_bytes`. Do not pass a MAC filter; the controller returns every client
present in that window, including clients that are currently offline.

JSON output sets `"backend": "legacy-controller"`.

## Counter semantics

| Command | Scope | Download | Upload |
|---------|-------|----------|--------|
| `traffic wan` | `wan` | `wan-rx_bytes` (+ `wan2-rx_bytes` when present) | `wan-tx_bytes` (+ `wan2-tx_bytes`) |
| `traffic clients` | `client-all-traffic` | `rx_bytes` + `wired-rx_bytes` when present | `tx_bytes` + `wired-tx_bytes` when present |

Client counters are LAN-inclusive (all traffic UniFi attributed to the client).
They are **not** Internet/WAN usage. WAN site totals are the Internet figures.

Missing attributes are omitted, never coerced to zero. Empty successful reports
yield `"totals": null` and `coverage.status=empty`. Denied endpoints exit `6`
with `"status":"denied"`; missing endpoints exit `11` with `"status":"unsupported"`.

Raw controller values may be fractional. unicli sums as `float64` and rounds
once after aggregation (`units.bytes=B`).

## Coverage

Resolutions are combined only across non-overlapping boundaries, coarsest first:

1. Complete local calendar days from `daily` (skipped on DST 23h/25h days)
2. Remaining holes from `hourly`
3. Remaining holes from `5minutes`

Incomplete current intervals (`bucket.end > observed_at`) are excluded.
Month-to-date therefore includes today’s completed hours and 5-minute slots.
Gaps in historical data are listed; totals in that case are a **lower bound**.

Client ranking uses the same selected windows. A client that does not appear in
a selected bucket contributes nothing for that window (offline / idle), which
is not a missing-data zero. Ranking is not derived from the connected-client
list or from lifetime/session counters on `rest/user`.

`--sort download|upload|total` (default `total`). `--top` 1–100 (default 10).
`--offset` paginates the ranking; `ranking.partial` is true when the page is
not the full ranked set. `--all` caps at 200 and sets `ranking.truncated`.

Stable client identity is MAC (`user` / `oid` in the report). Names/`id` come
from `rest/user` when available. `connected` is true only if the MAC is in the
Integration connected-client list; it is `null` if that list cannot be loaded.

## Supported versions and limits

Verified live against UniFi Network **10.6.101** on a UniFi OS console (API key,
TLS verification unchanged: `--insecure` / profile `insecure` as before).

Classic `stat/report` exists from controller 5.8+ for user history (Art-of-WiFi /
ubntwiki). Retention is controller-configured: 5-minute data is short, hourly
often about a week, daily covers a month-to-date window on the lab console.

Hourly and 5-minute queries use a shorter lookback (8 days and 24 hours)
clipped into the requested window so month-to-date still includes today's
completed intervals. A single month-long hourly.user query can omit today.

Limits: 32 calendar days, `--top` ≤ 100, `--all` ≤ 200 ranked clients.

Site scoping is the selected profile/site slug. Reports are not mixed across
sites.

## Live proof (lab, 2026-09-13)

Read-only probes against the configured console, no mutations:

- `GET /proxy/network/integration/v1/info` → `applicationVersion=10.6.101`
- Integration traffic/statistics/reports paths → HTTP 404 (no historical Integration API)
- v2 `/traffic`, `/country-traffic`, `/clients/history` → HTML 404
- `POST .../stat/report/5minutes.site` → 36 WAN buckets (`wan-rx_bytes` / `wan-tx_bytes`)
- `POST .../stat/report/hourly.site` → 48 WAN buckets
- `POST .../stat/report/daily.site` → 13 WAN buckets (month-to-date including today)
- `POST .../stat/report/5minutes.user` → 2641 rows, 77 distinct MACs
- `POST .../stat/report/hourly.user` → 3385 rows, 93 MACs
- `POST .../stat/report/daily.user` → 1108 rows, 99 MACs (includes clients absent from the last 3 hours)

Daily timestamps aligned to local midnight (`Europe/Warsaw`). `wired-*` attrs
were omitted by this firmware; `rx_bytes`/`tx_bytes` were populated for wired
and wireless clients.

Limitations seen live: no Integration historical traffic; v2 traffic-identification
API absent; user reports are LAN-inclusive; current 5-minute slot is incomplete
and excluded; hourly/5-minute retention is shorter than daily.
