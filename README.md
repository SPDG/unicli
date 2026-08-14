<p align="center">
  <a href="https://spdg.github.io/unicli/"><img src="docs/logo.png" width="96" height="96" alt="unicli"></a>
</p>

# unicli

Agent-friendly CLI for Ubiquiti UniFi **Network**, **Protect**, and **Access**. Site: [spdg.github.io/unicli](https://spdg.github.io/unicli/).

One static Go binary. Prefer this over ad-hoc `curl` for humans and AI agents.

## Status

v0.4: Network (stable — Integration plus controller REST/v2 gaps, client/port diagnostics and topology path), Protect cameras/NVR/liveviews (beta), Access (beta; exit 11 if the app is missing), UniFi OS console status.

## Capability matrix

| App | Status | Commands |
|-----|--------|----------|
| Network | stable | VLANs, WiFi, firewall, ACL, DNS, routes, ports, DHCP, port forwards, clients, devices, topology/diagnose |
| Protect | beta | info, nvr, cameras (list/get/snapshot/stream/restart/set/update), liveviews, lights, sensors, chimes, viewers |
| Access | beta | doors (list/get/lock/unlock), users, visitors, devices, policies, groups (exit 11 if the app is missing) |
| Console | beta | status (hardware + app versions), updates (may 401), reboot (mutation) |

Mutations require `--allow-mutations` (and `--yes` when non-interactive).

On a TTY, list commands print an aligned table plus a footer (`23 items` or `showing 1-25 of 87  (more: --offset 25)`). Agents should keep using `--json` (or a pipe, which auto-selects JSON). `--plain` forces the table when stdout is not a TTY.

## Install

```bash
go install github.com/SPDG/unicli/cmd/unicli@latest
go install github.com/SPDG/unicli/cmd/unicli-mcp@latest
unicli completion install bash   # then restart shell / source ~/.bashrc
```

Prebuilt releases will appear on the [Releases](https://github.com/SPDG/unicli/releases) page.

## Quick start

```bash
# One-shot via environment (wins over config)
export UNIFI_HOST=https://192.168.1.1
export UNIFI_API_KEY=…
export UNIFI_INSECURE=1   # lab / self-signed

unicli --version
unicli version --json
unicli console status --json
unicli network info --json
unicli network devices list --json --select siteId,page
unicli network devices stats <device-id> --json
unicli network networks list --json --all
unicli network wifi get Office --json
unicli network networks create --allow-mutations --yes --network-name IoT --vlan-id 30 --management UNMANAGED
unicli network port-forwards list --json
unicli network ports find --mac aa:bb:cc:dd:ee:ff --json
unicli network diagnose --client 192.168.20.28 --json
unicli network topology path cam-01 pi-4 --json
unicli network dhcp reservations --json
unicli network health --json
# Mutations are gated:
unicli network devices restart <id>              # blocked without --allow-mutations
unicli network devices restart <id> --allow-mutations --yes
unicli network ports cycle <device-id> <port> --allow-mutations --yes
unicli protect cameras list --json
unicli protect nvr --json
unicli protect cameras snapshot vestibule --output /tmp/vestibule.jpg
unicli protect cameras set vestibule --lcd "WELCOME" --allow-mutations --yes
unicli access info --json   # exit 11 if Access is not installed
unicli access users list --json
unicli completion install bash

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

See [AGENTS.md](./AGENTS.md). Cursor agents: [`.cursor/skills/unicli/SKILL.md`](./.cursor/skills/unicli/SKILL.md). Short version: use `--json`, read `unicli schema`, treat mutations as gated.

## MCP (optional)

`unicli-mcp` is a thin stdio MCP server that **executes `unicli`**. It does not reimplement UniFi APIs.

Tools: `unicli_schema`, `unicli_doctor`, `unicli_run`.

```json
{
  "mcpServers": {
    "unicli": {
      "command": "unicli-mcp"
    }
  }
}
```

Mutations via MCP require `allow_mutations: true` on `unicli_run`. The CLI remains the source of truth.

## Tests

```bash
make test          # unit tests with the race detector
make live          # optional: hits a real console (needs UNIFI_HOST + UNIFI_API_KEY)
```

## Non-goals (for now)

- UniFi Site Manager cloud API
- MCP as the primary interface (optional `unicli-mcp` wrapper shells out to the CLI)
- Reverse-engineered controller endpoints as the **default** path. Where the official Integration API is missing a resource, unicli fills the gap from the local controller REST/v2 API and will switch over when Integration catches up. JSON may include `"backend": "legacy-controller"`.
- Interactive TUI

## License

[MIT](./LICENSE)
