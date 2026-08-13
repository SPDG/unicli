package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
)

func newNetworkPortForwardsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forwards",
		Short: "NAT port forwards",
	}
	cmd.AddCommand(legacyListCmd("list", "List port forwards", "portforward",
		[]string{"NAME", "ENABLED", "DST", "FWD", "PROTO", "ID"},
		[]string{"name", "enabled", "dst_port", "fwd", "proto", "_id"}))
	cmd.AddCommand(legacyGetCmd("get <id-or-name>", "Get a port forward", "portforward"))
	cmd.AddCommand(legacyCreateCmd("create", "Create a port forward from JSON (mutation)", "network port-forwards create", "portforward"))
	cmd.AddCommand(legacyUpdateCmd("update <id-or-name>", "Update a port forward from JSON (mutation)", "network port-forwards update", "portforward"))
	cmd.AddCommand(legacyDeleteCmd("delete <id-or-name>", "Delete a port forward (mutation)", "network port-forwards delete", "portforward"))
	return cmd
}

func newNetworkDynamicDNSCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dynamic-dns", Short: "Dynamic DNS records"}
	cmd.AddCommand(legacyListCmd("list", "List Dynamic DNS records", "dynamicdns",
		[]string{"NAME", "ID"}, []string{"name", "_id"}))
	cmd.AddCommand(legacyGetCmd("get <id-or-name>", "Get a Dynamic DNS record", "dynamicdns"))
	cmd.AddCommand(legacyCreateCmd("create", "Create Dynamic DNS from JSON (mutation)", "network dynamic-dns create", "dynamicdns"))
	cmd.AddCommand(legacyUpdateCmd("update <id-or-name>", "Update Dynamic DNS from JSON (mutation)", "network dynamic-dns update", "dynamicdns"))
	cmd.AddCommand(legacyDeleteCmd("delete <id-or-name>", "Delete Dynamic DNS (mutation)", "network dynamic-dns delete", "dynamicdns"))
	return cmd
}

func newNetworkClientGroupsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "client-groups", Short: "Client / user groups"}
	cmd.AddCommand(legacyListCmd("list", "List client groups", "usergroup",
		[]string{"NAME", "ID"}, []string{"name", "_id"}))
	cmd.AddCommand(legacyGetCmd("get <id-or-name>", "Get a client group", "usergroup"))
	cmd.AddCommand(legacyCreateCmd("create", "Create a client group from JSON (mutation)", "network client-groups create", "usergroup"))
	cmd.AddCommand(legacyUpdateCmd("update <id-or-name>", "Update a client group from JSON (mutation)", "network client-groups update", "usergroup"))
	cmd.AddCommand(legacyDeleteCmd("delete <id-or-name>", "Delete a client group (mutation)", "network client-groups delete", "usergroup"))
	return cmd
}

func newNetworkHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Subsystem health (WLAN, WAN, LAN, VPN, …)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.StatHealth(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				page := network.SlicePage(items, 0, len(items), true)
				rows := rawRows(page.Data, "subsystem", "status", "num_user", "num_guest")
				return printList(cmd, legacyWrap("health", slug, page), []string{"SUBSYSTEM", "STATUS", "USERS", "GUESTS"}, rows, 0, len(rows), len(rows))
			})
		},
	}
}

func newNetworkDHCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dhcp",
		Short: "DHCP reservations (fixed IPs)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "reservations",
		Short: "List DHCP reservations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				users, err := api.LegacyList(cmd.Context(), slug, "user")
				if err != nil {
					return mapAPIErr(err)
				}
				var reserved []map[string]any
				for _, u := range users {
					if u["use_fixedip"] == true || anyString(u["fixed_ip"]) != "" {
						if rootOpts.filterName != "" && !matchName(anyString(u["name"])+anyString(u["hostname"])+anyString(u["mac"]), rootOpts.filterName) {
							continue
						}
						reserved = append(reserved, u)
					}
				}
				page := network.SlicePage(reserved, rootOpts.offset, rootOpts.limit, rootOpts.allPages)
				rows := make([][]string, 0, len(page.Data))
				for _, u := range page.Data {
					host := anyString(u["name"])
					if host == "" || host == "-" {
						host = anyString(u["hostname"])
					}
					rows = append(rows, []string{host, anyString(u["mac"]), anyString(u["fixed_ip"]), network.LegacyID(u)})
				}
				return printList(cmd, legacyWrap("dhcp-reservations", slug, page), []string{"NAME", "MAC", "IP", "ID"}, rows, page.Offset, len(page.Data), page.TotalCount)
			})
		},
	})
	var fromJSON string
	set := &cobra.Command{
		Use:   "reserve",
		Short: "Create or update a DHCP reservation from JSON (mutation)",
		Long:  "POST/PUT rest/user. JSON typically includes mac, fixed_ip, use_fixedip, name, network_id. If _id is set, the user record is updated.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				body, err := readJSONBody(fromJSON)
				if err != nil {
					return err
				}
				id := network.LegacyID(body)
				action := "network dhcp reserve"
				return runMutation(cmd, action, fmt.Sprintf("Set DHCP reservation for %v?", body["mac"]), func() (any, error) {
					if id == "" {
						out, err := api.LegacyCreate(cmd.Context(), slug, "user", body)
						return legacyWrap("dhcp-reservations", slug, out), err
					}
					out, err := api.LegacyUpdate(cmd.Context(), slug, "user", id, body)
					return legacyWrap("dhcp-reservations", slug, out), err
				})
			})
		},
	}
	jsonFlag(set, &fromJSON)
	_ = set.MarkFlagRequired("from-json")
	cmd.AddCommand(set)
	return cmd
}

func newPortsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [device]",
		Short: "List switch ports (optionally one device)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				devs, err := api.StatDevices(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				if len(args) == 1 {
					dev, err := findStatDevice(devs, args[0])
					if err != nil {
						return err
					}
					devs = []map[string]any{dev}
				}
				var flat []map[string]any
				for _, d := range devs {
					devName := anyString(d["name"])
					if devName == "" {
						devName = anyString(d["mac"])
					}
					ports, _ := d["port_table"].([]any)
					for _, raw := range ports {
						p, ok := raw.(map[string]any)
						if !ok {
							continue
						}
						if rootOpts.filterName != "" && !matchName(devName+" "+anyString(p["name"]), rootOpts.filterName) {
							continue
						}
						flat = append(flat, map[string]any{"device": devName, "deviceId": network.LegacyID(d), "port": p})
					}
				}
				page := network.SlicePage(flat, rootOpts.offset, rootOpts.limit, rootOpts.allPages)
				rows := make([][]string, 0, len(page.Data))
				for _, item := range page.Data {
					p, _ := item["port"].(map[string]any)
					rows = append(rows, []string{
						anyString(item["device"]),
						anyString(p["port_idx"]),
						anyString(p["name"]),
						anyString(p["up"]),
						anyString(p["speed"]),
						anyString(p["poe_mode"]),
						anyString(p["portconf_id"]),
					})
				}
				return printList(cmd, legacyWrap("device-ports", slug, page),
					[]string{"DEVICE", "PORT", "NAME", "UP", "SPEED", "POE", "PROFILE"},
					rows, page.Offset, len(rows), page.TotalCount)
			})
		},
	}
}

func newPortsSetCmd() *cobra.Command {
	var profile, poe, portName string
	cmd := &cobra.Command{
		Use:   "set <device> <port-idx>",
		Short: "Set switch port profile, PoE, or name (mutation)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			portIdx, err := strconv.Atoi(args[1])
			if err != nil || portIdx < 1 {
				return exitf(exitcode.Usage, "invalid port index %q", args[1])
			}
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				devs, err := api.StatDevices(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				dev, err := findStatDevice(devs, args[0])
				if err != nil {
					return err
				}
				patch := map[string]any{"port_idx": portIdx}
				if profile != "" {
					profiles, err := api.LegacyList(cmd.Context(), slug, "portconf")
					if err != nil {
						return mapAPIErr(err)
					}
					p, err := findLegacy(profiles, profile)
					if err != nil {
						return err
					}
					patch["portconf_id"] = network.LegacyID(p)
				}
				if poe != "" {
					patch["poe_mode"] = poe
				}
				if portName != "" {
					patch["name"] = portName
				}
				if len(patch) == 1 {
					return exitf(exitcode.Usage, "set at least one of --profile, --poe, --port-name")
				}
				overrides := mergePortOverride(dev["port_overrides"], portIdx, patch)
				id := network.LegacyID(dev)
				return runMutation(cmd, "network ports set",
					fmt.Sprintf("Update port %d on %s?", portIdx, anyString(dev["name"])),
					func() (any, error) {
						out, err := api.UpdateLegacyDevice(cmd.Context(), slug, id, map[string]any{"port_overrides": overrides})
						if err != nil {
							return nil, err
						}
						return legacyWrap("device-ports", slug, out), nil
					})
			})
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "port profile name or id")
	cmd.Flags().StringVar(&poe, "poe", "", "PoE mode: auto, passthrough, off")
	cmd.Flags().StringVar(&portName, "port-name", "", "port label")
	return cmd
}

func findStatDevice(devs []map[string]any, arg string) (map[string]any, error) {
	arg = strings.TrimSpace(arg)
	var exact, partial []map[string]any
	for _, d := range devs {
		id := network.LegacyID(d)
		name := anyString(d["name"])
		mac := anyString(d["mac"])
		if id == arg || mac == arg || name == arg {
			exact = append(exact, d)
			continue
		}
		if matchName(name, arg) || matchName(mac, arg) {
			partial = append(partial, d)
		}
	}
	hits := exact
	if len(hits) == 0 {
		hits = partial
	}
	if len(hits) == 0 {
		return nil, exitf(exitcode.NotFound, "no device matching %q", arg)
	}
	if len(hits) > 1 {
		return nil, exitf(exitcode.Usage, "ambiguous device %q (%d matches)", arg, len(hits))
	}
	return hits[0], nil
}

func mergePortOverride(existing any, portIdx int, patch map[string]any) []map[string]any {
	var out []map[string]any
	found := false
	if arr, ok := existing.([]any); ok {
		for _, raw := range arr {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			idx := int(asFloat(item["port_idx"]))
			if idx == portIdx {
				for k, v := range patch {
					item[k] = v
				}
				found = true
			}
			out = append(out, item)
		}
	}
	if !found {
		out = append(out, patch)
	}
	return out
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		n, _ := strconv.ParseFloat(fmt.Sprint(v), 64)
		return n
	}
}
