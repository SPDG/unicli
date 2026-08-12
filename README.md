# unicli

Agent-friendly CLI for Ubiquiti UniFi **Network**, **Protect**, and **Access**.

One static Go binary. Prefer this over ad-hoc `curl` for humans and AI agents.

## Status

Phase 1 (Network MVP): multi-gateway profiles, env overrides, `doctor`, and read-only Network commands.

Protect and Access come next.

## Install

```bash
go install github.com/SPDG/unicli/cmd/unicli@latest
```

Prebuilt releases will appear on the [Releases](https://github.com/SPDG/unicli/releases) page.

## Quick start

```bash
# One-shot via environment (wins over config)
export UNIFI_HOST=https://192.168.1.1
export UNIFI_API_KEY=…
export UNIFI_INSECURE=1   # lab / self-signed

unicli doctor --json
unicli network info --json
unicli network devices list --json
unicli network clients list --json --limit 50

# Multiple gateways via named profiles
printf %s "$UNIFI_API_KEY" | unicli auth login --profile home --host https://192.168.1.1 --insecure
unicli profile set office https://unifi.office.example
printf %s "$OFFICE_KEY" | unicli auth login --profile office
unicli profile use office
unicli --profile home network sites list --json
```

Resolution order: CLI flags → env (`UNIFI_HOST`, `UNIFI_API_KEY`, `UNIFI_PROFILE`, `UNIFI_INSECURE`, `UNIFI_SITE`) → config `current` profile.

API keys are never accepted on argv (they leak via process lists and shell history). Keys for profiles live in `~/.config/unicli/credentials.json` (mode `0600`).

## Agent usage

See [AGENTS.md](./AGENTS.md). Short version: use `--json`, read `unicli schema`, treat mutations as gated.

## Non-goals (for now)

- UniFi Site Manager cloud API
- MCP as the primary interface (optional thin wrapper may come later)
- Reverse-engineered undocumented controller endpoints as the default path
- Interactive TUI

## License

[MIT](./LICENSE)
