# Agent guide for unicli

This CLI is designed so agents can call UniFi APIs without hand-rolled `curl`.

The Cursor skill with the same rules lives at [`.cursor/skills/unicli/SKILL.md`](./.cursor/skills/unicli/SKILL.md). Load that skill when querying or mutating UniFi through unicli.

## Rules

1. Prefer `unicli` over raw HTTP when a command exists.
2. Always pass `--json` for machine parsing (non-TTY stdout also defaults to JSON). Human TTY output is a table with a pagination footer.
3. Discover the surface with `unicli schema --json` and `unicli --help` before guessing flags.
4. Never put API keys on the command line. Use `UNIFI_API_KEY` or `unicli auth login` (key on stdin).
5. Assume read-only by default. Mutations require `--allow-mutations`; destructive actions also need `--yes` when stdin is not a TTY.
6. Use `--select a,b.c` to keep JSON payloads small.
7. If Access is not installed on the console, Access commands exit `11` (`unsupported`) instead of inventing data.
8. For multi-gateway setups, set `UNIFI_PROFILE` or `--profile`, or rely on config `current`. Env host/key overrides the selected profile.
9. Branch on exit codes from `unicli schema`, not on scraped stderr prose.
10. Optional MCP: `unicli-mcp` exposes `unicli_schema`, `unicli_doctor`, and `unicli_run`. Prefer the CLI when a shell is available.

Prefer Integration-backed commands when both exist. Other Network commands fill controller REST/v2 gaps until Integration catches up. Users should treat the CLI as one surface; JSON may include `"backend":"legacy-controller"`.

## Typical flow

```bash
unicli doctor --json
unicli schema --json
unicli network devices list --json --limit 50
unicli network networks list --json --all
unicli network wifi get Office --json
unicli network firewall zones list --json
unicli network firewall policies list --json --all
unicli network ports list --json --all
unicli network dhcp reservations --json
unicli network port-forwards list --json
unicli network routes list --json
unicli network port-profiles list --json
unicli network lists list --json
unicli protect nvr --json
unicli protect cameras snapshot vestibule --output /tmp/cam.jpg
unicli protect cameras set vestibule --hdr on --allow-mutations --yes
```

## Configuration

| Source | Variables / keys |
|--------|------------------|
| Env | `UNIFI_HOST`, `UNIFI_API_KEY`, `UNIFI_PROFILE`, `UNIFI_INSECURE`, `UNIFI_SITE` |
| Config | `~/.config/unicli/config.yaml` — `current` + `profiles.<name>.{host,insecure,site}` |
| Secrets | `~/.config/unicli/credentials.json` (0600), keyed by profile name |

Precedence: flags → env → selected profile (`--profile` / `UNIFI_PROFILE` / `current`).
