# Agent guide for unicli

This CLI is designed so agents can call UniFi APIs without hand-rolled `curl`.

## Rules

1. Prefer `unicli` over raw HTTP when a command exists.
2. Always pass `--json` (or rely on non-TTY auto-JSON once implemented) for machine parsing.
3. Discover the surface with `unicli schema --json` and `unicli --help` before guessing flags.
4. Never put API keys on the command line. Use `UNIFI_API_KEY` or `unicli auth login` (key on stdin).
5. Assume read-only by default. Mutations require `--allow-mutations`; destructive actions also need `--yes` when stdin is not a TTY.
6. Use `--select a,b.c` to keep JSON payloads small.
7. If Access is not installed on the console, Access commands exit `11` (`unsupported`) instead of inventing data.
8. For multi-gateway setups, set `UNIFI_PROFILE` or `--profile`, or rely on config `current`. Env host/key overrides the selected profile.
9. Branch on exit codes from `unicli schema`, not on scraped stderr prose.

## Typical flow

```bash
unicli doctor --json
unicli schema --json
unicli network devices list --json --limit 50
```

## Configuration

| Source | Variables / keys |
|--------|------------------|
| Env | `UNIFI_HOST`, `UNIFI_API_KEY`, `UNIFI_PROFILE`, `UNIFI_INSECURE`, `UNIFI_SITE` |
| Config | `~/.config/unicli/config.yaml` — `current` + `profiles.<name>.{host,insecure,site}` |
| Secrets | `~/.config/unicli/credentials.json` (0600), keyed by profile name |

Precedence: flags → env → selected profile (`--profile` / `UNIFI_PROFILE` / `current`).
