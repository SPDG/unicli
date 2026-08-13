# unicli

Agent-friendly CLI for Ubiquiti UniFi **Network**, **Protect**, and **Access**.

One static Go binary. Prefer this over ad-hoc `curl` for humans and AI agents.

## Status

Phase 2: Network read + gated mutations (`restart`, port `POWER_CYCLE`, guest authorize), `--select`, goreleaser.

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
unicli network devices list --json --select siteId,page
unicli network devices stats <device-id> --json
# Mutations are gated:
unicli network devices restart <id>              # blocked without --allow-mutations
unicli network devices restart <id> --allow-mutations --yes
unicli network ports cycle <device-id> <port> --allow-mutations --yes

# Multiple gateways via named profiles
printf %s "$UNIFI_API_KEY" | unicli auth login --profile home --host https://192.168.1.1 --insecure
unicli profile use home
```

Resolution order: CLI flags → env (`UNIFI_HOST`, `UNIFI_API_KEY`, `UNIFI_PROFILE`, `UNIFI_INSECURE`, `UNIFI_SITE`) → config `current` profile.

API keys are never accepted on argv (they leak via process lists and shell history). Keys for profiles live in `~/.config/unicli/credentials.json` (mode `0600`).

## Safety

- Read-only by default.
- Mutations require `--allow-mutations`.
- Destructive mutations also require interactive confirmation, or `--yes` when stdin is not a TTY.

## Agent usage

See [AGENTS.md](./AGENTS.md). Short version: use `--json`, read `unicli schema`, treat mutations as gated.

## Non-goals (for now)

- UniFi Site Manager cloud API
- MCP as the primary interface (optional thin wrapper may come later)
- Reverse-engineered undocumented controller endpoints as the default path
- Interactive TUI

## License

[MIT](./LICENSE)
