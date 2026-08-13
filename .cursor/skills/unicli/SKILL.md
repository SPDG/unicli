---
name: unicli
description: >-
  Call Ubiquiti UniFi Network, Protect, and Access through the unicli CLI instead of
  curl or undocumented controller APIs. Use when querying or mutating UniFi (devices,
  clients, VLANs/networks, WiFi, firewall, ACL, cameras, doors), when the user mentions
  unicli, UniFi, UDM, or UniFi OS, or when writing scripts/agents against a local console.
---

# unicli

Agent-friendly CLI for UniFi OS consoles. Prefer `unicli` over raw HTTP when a command exists.

Discover live flags and commands with `unicli schema --json` before guessing. This skill can lag the binary.

## Hard rules

1. Always pass `--json` (or rely on non-TTY stdout, which defaults to JSON). Do not parse human tables.
2. Never put API keys on argv. Use `UNIFI_API_KEY` or `printf %s "$KEY" | unicli auth login`.
3. Read-only by default. Mutations need `--allow-mutations`. Destructive mutations also need `--yes` when stdin is not a TTY.
4. Do not pass `--include-secrets` unless the user explicitly needs a passphrase or RTSPS token. `network wifi get` and `protect cameras stream` redact secrets by default.
5. If Access is missing, commands exit `11` (`unsupported`). Do not invent doors/users.
6. Branch on exit codes from `unicli schema`, not on stderr prose.
7. Use `--select a,b.c` to keep payloads small. Use `--limit` / `--offset` and read `page.totalCount`.

## First run

```bash
unicli doctor --json
unicli schema --json
```

Then the smallest command that answers the question, for example:

```bash
unicli network clients list --json --limit 50
unicli network devices list --json --select siteId,page
unicli network networks list --json
unicli network wifi list --json
unicli network firewall zones list --json
unicli network firewall policies list --json --limit 50
```

If `doctor` fails, fix auth/config. Do not fall back to `curl` for Integration API paths that unicli already wraps.

## Config

| Source | Keys |
|--------|------|
| Env | `UNIFI_HOST`, `UNIFI_API_KEY`, `UNIFI_PROFILE`, `UNIFI_INSECURE`, `UNIFI_SITE` |
| Config | `~/.config/unicli/config.yaml` — `current` + `profiles.<name>.{host,insecure,site}` |
| Secrets | `~/.config/unicli/credentials.json` (0600), keyed by profile name |

Precedence: flags → env → selected profile (`--profile` / `UNIFI_PROFILE` / `current`).

Multi-gateway: set `--profile` / `UNIFI_PROFILE`, or `unicli profile use <name>`.

## Pagination

List JSON includes `page.offset`, `page.limit`, `page.count`, `page.totalCount`, `page.data`.

If `offset + count < totalCount`, fetch the next page with `--offset`, or pass `--all`. Default `--limit` is 25. Filter with `--name`, `--vlan`, `--enabled`. Human TTY tables print a footer; agents must use JSON.

## Commands (Network-first)

| Task | Command |
|------|---------|
| VLANs / L3 networks | `network networks list\|get\|create\|update\|enable\|disable\|delete\|references` |
| WiFi SSIDs | `network wifi list\|get\|create\|update\|enable\|disable\|delete` |
| Firewall zones / policies | `network firewall zones …`, `network firewall policies …` (incl. `logging` PATCH) |
| ACL / DNS / vouchers / matching lists | `network acl`, `network dns`, `network vouchers`, `network matching-lists` |
| Static routes / PBR / port profiles / IP groups | `network routes`, `network traffic-routes`, `network port-profiles`, `network lists` |
| Port forwards / switch ports / DHCP | `network port-forwards`, `network ports list\|set`, `network dhcp reservations` |
| Health / DDNS / client groups | `network health`, `network dynamic-dns`, `network client-groups` |
| Read-only extras | `network vpn`, `network wans`, `network dpi`, `network radius`, `network switching`, `network tags`, `network pending-devices` |
| Clients | `network clients list\|get` |
| Devices | `network devices list\|get\|stats` |
| Sites / app info | `network sites list`, `network info` |
| Protect cameras / NVR / liveviews | `protect cameras …` (incl. `snapshot`, `stream`, `set`, `update`), `protect nvr`, `protect liveviews` |
| Protect extras | `protect lights`, `protect sensors`, `protect chimes`, `protect viewers` (empty lists if none) |
| Access | `access doors` (lock/unlock), `access users`, `access visitors`, `access devices`, `access policies`, `access door-groups`, `access user-groups` (exit 11 if Access is missing) |

Some commands fill Integration-API gaps via the local controller REST/v2 API. The CLI surface stays the same; JSON may include `"backend":"legacy-controller"` so agents can tell. Prefer Integration resources when both exist (VLANs, WiFi broadcasts, matching-lists). Firewall policy **list/get** uses v2 so every policy has an id and hit counters (Integration list often omits custom UUIDs).

Mutations need `--allow-mutations` and `--yes` when non-interactive. Create/update of complex DTOs (WiFi, firewall policy, DHCP) use `--from-json` (object, file, or `-`). Do not put passphrases or API keys on argv. `enable`/`disable` GET the object, drop read-only fields, and PUT. WiFi passphrases and Protect RTSPS tokens stay redacted unless `--include-secrets`. `protect cameras snapshot` writes JPEG to `--output` (or stdout with `-o -`).

## Exit codes

| Code | Name |
|------|------|
| 0 | ok |
| 2 | usage |
| 3 | empty |
| 4 | auth_required |
| 5 | not_found |
| 6 | permission |
| 7 | rate_limited |
| 8 | retryable |
| 10 | config |
| 11 | unsupported (app missing) |
| 12 | mutation_blocked |
| 13 | input_required |
| 130 | cancelled |

## MCP

`unicli-mcp` is a thin stdio wrapper that execs `unicli`. Prefer the CLI when a shell is available. Mutations via MCP require `allow_mutations: true` on `unicli_run`.
