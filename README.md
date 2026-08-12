# unicli

Agent-friendly CLI for Ubiquiti UniFi **Network**, **Protect**, and **Access**.

One static Go binary. Prefer this over ad-hoc `curl` for humans and AI agents.

## Status

Early scaffold (Phase 0). Network/Protect/Access commands land in upcoming releases.

## Install

```bash
go install github.com/SPDG/unicli/cmd/unicli@latest
```

Prebuilt releases will appear on the [Releases](https://github.com/SPDG/unicli/releases) page.

## Quick start (planned)

```bash
# One-shot via environment (highest priority)
export UNIFI_HOST=https://192.168.1.1
export UNIFI_API_KEY=…
export UNIFI_INSECURE=1   # lab / self-signed

unicli doctor --json
unicli network devices list --json

# Or multiple gateways via named profiles in ~/.config/unicli/config.yaml
unicli profile use office
unicli --profile lab network clients list --json
```

Resolution order: CLI flags → env (`UNIFI_HOST`, `UNIFI_API_KEY`, `UNIFI_PROFILE`, `UNIFI_INSECURE`) → config `current` profile.

API keys are never accepted on argv (they leak via process lists and shell history).

## Agent usage

See [AGENTS.md](./AGENTS.md). Short version: use `--json`, read `unicli schema`, treat mutations as gated.

## Non-goals (for now)

- UniFi Site Manager cloud API
- MCP as the primary interface (optional thin wrapper may come later)
- Reverse-engineered undocumented controller endpoints as the default path
- Interactive TUI

## License

[MIT](./LICENSE)
