package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
	"github.com/SPDG/unicli/internal/redact"
)

func maybeRedact(v any) any {
	if rootOpts.includeSecrets {
		return v
	}
	return redact.JSON(v)
}

func runMutation(cmd *cobra.Command, action, confirm string, fn func() (any, error)) error {
	if err := requireMutations(action); err != nil {
		return err
	}
	if err := requireConfirm(confirm); err != nil {
		return err
	}
	out, err := fn()
	if err != nil {
		return mapAPIErr(err)
	}
	if out == nil {
		out = map[string]any{"status": "ok"}
	}
	return printValue(cmd, maybeRedact(out), nil)
}

func jsonFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVar(dest, "from-json", "", "JSON body: object, file path, @file, or - for stdin")
}

func forceFlag(cmd *cobra.Command, dest *bool) {
	cmd.Flags().BoolVar(dest, "force", false, "pass force=true to the UniFi delete API")
}

func newNetworkNetworksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "networks",
		Aliases: []string{"vlans"},
		Short:   "Layer-3 networks / VLANs",
	}
	cmd.AddCommand(newNetworksListCmd())
	cmd.AddCommand(newNetworksGetCmd())
	cmd.AddCommand(newNetworksCreateCmd())
	cmd.AddCommand(newNetworksUpdateCmd())
	cmd.AddCommand(newNetworksEnableCmd(true))
	cmd.AddCommand(newNetworksEnableCmd(false))
	cmd.AddCommand(newNetworksDeleteCmd())
	cmd.AddCommand(newNetworksRefsCmd())
	return cmd
}

func newNetworksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List networks (VLANs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[network.Network], error) {
					return api.Networks(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				en, err := parseEnabledFilter()
				if err != nil {
					return err
				}
				out := map[string]any{"siteId": siteID, "page": page}
				rows := make([][]string, 0, len(page.Data))
				for _, n := range page.Data {
					if !matchName(n.Name, rootOpts.filterName) {
						continue
					}
					if rootOpts.filterVLAN >= 0 && n.VlanID != rootOpts.filterVLAN {
						continue
					}
					if en != nil && n.Enabled != *en {
						continue
					}
					rows = append(rows, []string{n.Name, itoa(n.VlanID), yesNo(n.Enabled), n.Management, n.ID})
				}
				return printList(cmd, out, []string{"NAME", "VLAN", "ENABLED", "MGMT", "ID"}, rows, page.Offset, len(rows), page.TotalCount)
			})
		},
	}
}

func newNetworksGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <network-id-or-name>",
		Short: "Get a network / VLAN",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveNetworkID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				n, err := api.Network(cmd.Context(), siteID, id)
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, maybeRedact(n), nil)
			})
		},
	}
}

func newNetworksCreateCmd() *cobra.Command {
	var fromJSON, name, management, hostIP, zoneID string
	var vlan, prefix int
	var isolation bool
	internet := true
	enabled := true
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a network / VLAN (mutation)",
		Long:  "Create a VLAN. Prefer --from-json for full DHCP/NAT. Convenience flags cover a simple GATEWAY or UNMANAGED network.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := networkCreateBody(fromJSON, name, management, hostIP, zoneID, vlan, prefix, enabled, isolation, internet)
				if err != nil {
					return exitf(exitcode.Usage, "%v", err)
				}
				return runMutation(cmd, "network networks create", fmt.Sprintf("Create network %v (vlan %v)?", body["name"], body["vlanId"]), func() (any, error) {
					return api.CreateNetwork(cmd.Context(), siteID, body)
				})
			})
		},
	}
	jsonFlag(cmd, &fromJSON)
	cmd.Flags().StringVar(&name, "network-name", "", "network name (convenience create)")
	cmd.Flags().StringVar(&management, "management", "GATEWAY", "GATEWAY, UNMANAGED, or SWITCH")
	cmd.Flags().IntVar(&vlan, "vlan-id", 0, "VLAN ID (>=2 for additional networks)")
	cmd.Flags().StringVar(&hostIP, "host-ip", "", "gateway host IP (GATEWAY management)")
	cmd.Flags().IntVar(&prefix, "prefix-length", 24, "IPv4 prefix length")
	cmd.Flags().StringVar(&zoneID, "zone-id", "", "firewall zone UUID")
	cmd.Flags().BoolVar(&enabled, "set-enabled", true, "create as enabled")
	cmd.Flags().BoolVar(&isolation, "isolation", false, "isolate from other networks")
	cmd.Flags().BoolVar(&internet, "internet", true, "allow internet access")
	return cmd
}

func networkCreateBody(fromJSON, name, management, hostIP, zoneID string, vlan, prefix int, enabled, isolation, internet bool) (map[string]any, error) {
	if fromJSON != "" {
		return readJSONBody(fromJSON)
	}
	if name == "" || vlan == 0 {
		return nil, fmt.Errorf("create requires --from-json, or --network-name and --vlan-id")
	}
	body := map[string]any{
		"name":       name,
		"vlanId":     vlan,
		"enabled":    enabled,
		"management": management,
	}
	if management == "GATEWAY" || management == "" {
		body["management"] = "GATEWAY"
		body["isolationEnabled"] = isolation
		body["internetAccessEnabled"] = internet
		body["cellularBackupEnabled"] = false
		if hostIP == "" {
			return nil, fmt.Errorf("GATEWAY create requires --host-ip (or --from-json)")
		}
		body["ipv4Configuration"] = map[string]any{
			"autoScaleEnabled": false,
			"hostIpAddress":    hostIP,
			"prefixLength":     prefix,
		}
	}
	if zoneID != "" {
		body["zoneId"] = zoneID
	}
	return body, nil
}

func newNetworksUpdateCmd() *cobra.Command {
	var fromJSON string
	cmd := &cobra.Command{
		Use:   "update <network-id-or-name>",
		Short: "Update a network from JSON (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(fromJSON)
				if err != nil {
					return err
				}
				id, err := resolveNetworkID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				return runMutation(cmd, "network networks update", fmt.Sprintf("Update network %s?", args[0]), func() (any, error) {
					return api.UpdateNetwork(cmd.Context(), siteID, id, body)
				})
			})
		},
	}
	jsonFlag(cmd, &fromJSON)
	_ = cmd.MarkFlagRequired("from-json")
	return cmd
}

func newNetworksEnableCmd(enable bool) *cobra.Command {
	use, short, action := "disable", "Disable a network (mutation)", "network networks disable"
	if enable {
		use, short, action = "enable", "Enable a network (mutation)", "network networks enable"
	}
	return &cobra.Command{
		Use:   use + " <network-id-or-name>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveNetworkID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				return runMutation(cmd, action, fmt.Sprintf("%s network %s?", use, args[0]), func() (any, error) {
					cur, err := api.Network(cmd.Context(), siteID, id)
					if err != nil {
						return nil, err
					}
					body := network.DropReadOnly(cur)
					body["enabled"] = enable
					return api.UpdateNetwork(cmd.Context(), siteID, id, body)
				})
			})
		},
	}
}

func newNetworksDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <network-id-or-name>",
		Short: "Delete a network / VLAN (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveNetworkID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				return runMutation(cmd, "network networks delete", fmt.Sprintf("Delete network %s?", args[0]), func() (any, error) {
					if err := api.DeleteNetwork(cmd.Context(), siteID, id, force); err != nil {
						return nil, err
					}
					return map[string]any{"status": "ok", "id": id}, nil
				})
			})
		},
	}
	forceFlag(cmd, &force)
	return cmd
}

func newNetworksRefsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "references <network-id-or-name>",
		Short: "Show what references a network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveNetworkID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				n, err := api.NetworkReferences(cmd.Context(), siteID, id)
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, n, nil)
			})
		},
	}
}

func resolveNetworkID(cmd *cobra.Command, api *network.API, siteID, arg string) (string, error) {
	if isUUID(arg) {
		return arg, nil
	}
	page, err := collectPage(func(offset, limit int) (*network.Page[network.Network], error) {
		return api.Networks(cmd.Context(), siteID, offset, limit)
	})
	if err != nil {
		return arg, nil
	}
	return resolveID(arg, page.Data, func(n network.Network) string { return n.Name }, func(n network.Network) string { return n.ID })
}

func newNetworkWifiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wifi",
		Short: "WiFi broadcasts (SSIDs)",
	}
	cmd.AddCommand(newWifiListCmd())
	cmd.AddCommand(newWifiGetCmd())
	cmd.AddCommand(newWifiCreateCmd())
	cmd.AddCommand(newWifiUpdateCmd())
	cmd.AddCommand(newWifiEnableCmd(true))
	cmd.AddCommand(newWifiEnableCmd(false))
	cmd.AddCommand(newWifiDeleteCmd())
	return cmd
}

func newWifiListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List WiFi broadcasts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[network.WifiBroadcast], error) {
					return api.WifiBroadcasts(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				en, err := parseEnabledFilter()
				if err != nil {
					return err
				}
				out := map[string]any{"siteId": siteID, "page": page}
				rows := make([][]string, 0, len(page.Data))
				for _, w := range page.Data {
					if !matchName(w.Name, rootOpts.filterName) {
						continue
					}
					if en != nil && w.Enabled != *en {
						continue
					}
					rows = append(rows, []string{w.Name, yesNo(w.Enabled), w.Type, w.SecurityConfiguration.Type, w.ID})
				}
				return printList(cmd, out, []string{"NAME", "ENABLED", "TYPE", "SECURITY", "ID"}, rows, page.Offset, len(rows), page.TotalCount)
			})
		},
	}
}

func newWifiGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <wifi-id-or-name>",
		Short: "Get a WiFi broadcast (passphrases redacted unless --include-secrets)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveWifiID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				w, err := api.WifiBroadcast(cmd.Context(), siteID, id)
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, maybeRedact(w), nil)
			})
		},
	}
}

func newWifiCreateCmd() *cobra.Command {
	var fromJSON string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a WiFi broadcast from JSON (mutation)",
		Long:  "The Integration create DTO is large (security, radios, filters). Pass a JSON object via --from-json. Do not put passphrases on argv.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(fromJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network wifi create", fmt.Sprintf("Create WiFi %v?", body["name"]), func() (any, error) {
					return api.CreateWifiBroadcast(cmd.Context(), siteID, body)
				})
			})
		},
	}
	jsonFlag(cmd, &fromJSON)
	_ = cmd.MarkFlagRequired("from-json")
	return cmd
}

func newWifiUpdateCmd() *cobra.Command {
	var fromJSON string
	cmd := &cobra.Command{
		Use:   "update <wifi-id-or-name>",
		Short: "Replace a WiFi broadcast from JSON (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(fromJSON)
				if err != nil {
					return err
				}
				id, err := resolveWifiID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				return runMutation(cmd, "network wifi update", fmt.Sprintf("Update WiFi %s?", args[0]), func() (any, error) {
					return api.UpdateWifiBroadcast(cmd.Context(), siteID, id, body)
				})
			})
		},
	}
	jsonFlag(cmd, &fromJSON)
	_ = cmd.MarkFlagRequired("from-json")
	return cmd
}

func newWifiEnableCmd(enable bool) *cobra.Command {
	use, short, action := "disable", "Disable a WiFi broadcast (mutation)", "network wifi disable"
	if enable {
		use, short, action = "enable", "Enable a WiFi broadcast (mutation)", "network wifi enable"
	}
	return &cobra.Command{
		Use:   use + " <wifi-id-or-name>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveWifiID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				return runMutation(cmd, action, fmt.Sprintf("%s WiFi %s?", use, args[0]), func() (any, error) {
					cur, err := api.WifiBroadcast(cmd.Context(), siteID, id)
					if err != nil {
						return nil, err
					}
					body := network.DropReadOnly(cur)
					body["enabled"] = enable
					return api.UpdateWifiBroadcast(cmd.Context(), siteID, id, body)
				})
			})
		},
	}
}

func newWifiDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <wifi-id-or-name>",
		Short: "Delete a WiFi broadcast (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveWifiID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				return runMutation(cmd, "network wifi delete", fmt.Sprintf("Delete WiFi %s?", args[0]), func() (any, error) {
					if err := api.DeleteWifiBroadcast(cmd.Context(), siteID, id, force); err != nil {
						return nil, err
					}
					return map[string]any{"status": "ok", "id": id}, nil
				})
			})
		},
	}
	forceFlag(cmd, &force)
	return cmd
}

func resolveWifiID(cmd *cobra.Command, api *network.API, siteID, arg string) (string, error) {
	if isUUID(arg) {
		return arg, nil
	}
	page, err := collectPage(func(offset, limit int) (*network.Page[network.WifiBroadcast], error) {
		return api.WifiBroadcasts(cmd.Context(), siteID, offset, limit)
	})
	if err != nil {
		return arg, nil
	}
	return resolveID(arg, page.Data, func(w network.WifiBroadcast) string { return w.Name }, func(w network.WifiBroadcast) string { return w.ID })
}

func newNetworkFirewallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Firewall zones and policies",
	}
	cmd.AddCommand(newFirewallZonesCmd())
	cmd.AddCommand(newFirewallPoliciesCmd())
	return cmd
}

func newFirewallZonesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "zones",
		Short: "Firewall zones",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List firewall zones",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[network.FirewallZone], error) {
					return api.FirewallZones(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				out := map[string]any{"siteId": siteID, "page": page}
				rows := make([][]string, 0, len(page.Data))
				for _, z := range page.Data {
					if !matchName(z.Name, rootOpts.filterName) {
						continue
					}
					rows = append(rows, []string{z.Name, z.ID})
				}
				return printList(cmd, out, []string{"NAME", "ID"}, rows, page.Offset, len(rows), page.TotalCount)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <zone-id-or-name>",
		Short: "Get a firewall zone",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveZoneID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				z, err := api.FirewallZone(cmd.Context(), siteID, id)
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, z, nil)
			})
		},
	})
	var createJSON string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a custom firewall zone (mutation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(createJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network firewall zones create", fmt.Sprintf("Create firewall zone %v?", body["name"]), func() (any, error) {
					return api.CreateFirewallZone(cmd.Context(), siteID, body)
				})
			})
		},
	}
	jsonFlag(create, &createJSON)
	_ = create.MarkFlagRequired("from-json")
	cmd.AddCommand(create)

	var updateJSON string
	update := &cobra.Command{
		Use:   "update <zone-id-or-name>",
		Short: "Update a firewall zone (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(updateJSON)
				if err != nil {
					return err
				}
				id, err := resolveZoneID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				return runMutation(cmd, "network firewall zones update", fmt.Sprintf("Update firewall zone %s?", args[0]), func() (any, error) {
					return api.UpdateFirewallZone(cmd.Context(), siteID, id, body)
				})
			})
		},
	}
	jsonFlag(update, &updateJSON)
	_ = update.MarkFlagRequired("from-json")
	cmd.AddCommand(update)

	var force bool
	del := &cobra.Command{
		Use:   "delete <zone-id-or-name>",
		Short: "Delete a custom firewall zone (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				id, err := resolveZoneID(cmd, api, siteID, args[0])
				if err != nil {
					return err
				}
				return runMutation(cmd, "network firewall zones delete", fmt.Sprintf("Delete firewall zone %s?", args[0]), func() (any, error) {
					if err := api.DeleteFirewallZone(cmd.Context(), siteID, id, force); err != nil {
						return nil, err
					}
					return map[string]any{"status": "ok", "id": id}, nil
				})
			})
		},
	}
	forceFlag(del, &force)
	cmd.AddCommand(del)
	return cmd
}

func resolveZoneID(cmd *cobra.Command, api *network.API, siteID, arg string) (string, error) {
	if isUUID(arg) {
		return arg, nil
	}
	page, err := collectPage(func(offset, limit int) (*network.Page[network.FirewallZone], error) {
		return api.FirewallZones(cmd.Context(), siteID, offset, limit)
	})
	if err != nil {
		return arg, nil
	}
	return resolveID(arg, page.Data, func(z network.FirewallZone) string { return z.Name }, func(z network.FirewallZone) string { return z.ID })
}

func newFirewallPoliciesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policies",
		Short: "Firewall policies",
		Long:  "List and mutate zone-based firewall policies. The UI 'ID' column is policy index (30000 = built-in, 10000 = custom). Catch-all Allow/Block All use index 2147483647 (shown as default / an info icon in the UI). List/get use the controller v2 API so every policy has an id and hit counters. Create still uses Integration. Update/enable/disable/delete accept a v2 _id, name, or Integration UUID.",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List firewall policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.V2FirewallPolicies(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				en, err := parseEnabledFilter()
				if err != nil {
					return err
				}
				var filtered []map[string]any
				for _, p := range items {
					name := anyString(p["name"])
					if !matchName(name, rootOpts.filterName) {
						continue
					}
					if en != nil {
						if b, ok := p["enabled"].(bool); ok && b != *en {
							continue
						}
					}
					filtered = append(filtered, p)
				}
				page := network.SlicePage(filtered, rootOpts.offset, rootOpts.limit, rootOpts.allPages)
				rows := make([][]string, 0, len(page.Data))
				for _, p := range page.Data {
					idx := anyString(p["index"])
					if int(asFloat(p["index"])) == network.CatchAllPolicyIndex {
						idx = "default"
					}
					origin := "custom"
					if p["predefined"] == true {
						origin = "system"
					}
					rows = append(rows, []string{
						idx,
						anyString(p["name"]),
						anyString(p["action"]),
						origin,
						anyString(p["enabled"]),
						anyString(p["hits"]),
						network.LegacyID(p),
					})
				}
				out := legacyWrap("firewall-policies", slug, page)
				return printList(cmd, out, []string{"INDEX", "NAME", "ACTION", "ORIGIN", "ENABLED", "HITS", "ID"}, rows, page.Offset, len(rows), page.TotalCount)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <policy-id-or-name>",
		Short: "Get a firewall policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				if isUUID(args[0]) {
					id, err := resolveSiteID(cmd.Context(), api, "")
					if err == nil {
						p, err := api.FirewallPolicy(cmd.Context(), id, args[0])
						if err == nil {
							return printValue(cmd, p, nil)
						}
					}
				}
				items, err := api.V2FirewallPolicies(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				item, err := findLegacy(items, args[0])
				if err != nil {
					return err
				}
				detail, err := api.V2FirewallPolicy(cmd.Context(), slug, network.LegacyID(item))
				if err != nil {
					return printValue(cmd, legacyWrap("firewall-policies", slug, item), nil)
				}
				return printValue(cmd, legacyWrap("firewall-policies", slug, detail), nil)
			})
		},
	})
	var createJSON string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a firewall policy from JSON (mutation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(createJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network firewall policies create", fmt.Sprintf("Create firewall policy %v?", body["name"]), func() (any, error) {
					return api.CreateFirewallPolicy(cmd.Context(), siteID, body)
				})
			})
		},
	}
	jsonFlag(create, &createJSON)
	_ = create.MarkFlagRequired("from-json")
	cmd.AddCommand(create)

	var updateJSON string
	update := &cobra.Command{
		Use:   "update <policy-id-or-name>",
		Short: "Replace a firewall policy from JSON (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				body, err := readJSONBody(updateJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network firewall policies update", fmt.Sprintf("Update firewall policy %s?", args[0]), func() (any, error) {
					if isUUID(args[0]) {
						siteID, err := resolveSiteID(cmd.Context(), api, "")
						if err != nil {
							return nil, err
						}
						return api.UpdateFirewallPolicy(cmd.Context(), siteID, args[0], body)
					}
					items, err := api.V2FirewallPolicies(cmd.Context(), slug)
					if err != nil {
						return nil, err
					}
					item, err := findLegacy(items, args[0])
					if err != nil {
						return nil, err
					}
					return api.UpdateV2FirewallPolicy(cmd.Context(), slug, network.LegacyID(item), body)
				})
			})
		},
	}
	jsonFlag(update, &updateJSON)
	_ = update.MarkFlagRequired("from-json")
	cmd.AddCommand(update)

	cmd.AddCommand(newPolicyLoggingCmd())
	cmd.AddCommand(newPolicyEnableCmd(true))
	cmd.AddCommand(newPolicyEnableCmd(false))

	var force bool
	del := &cobra.Command{
		Use:   "delete <policy-id-or-name>",
		Short: "Delete a firewall policy (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				return runMutation(cmd, "network firewall policies delete", fmt.Sprintf("Delete firewall policy %s?", args[0]), func() (any, error) {
					if isUUID(args[0]) {
						siteID, err := resolveSiteID(cmd.Context(), api, "")
						if err != nil {
							return nil, err
						}
						if err := api.DeleteFirewallPolicy(cmd.Context(), siteID, args[0], force); err != nil {
							return nil, err
						}
						return map[string]any{"status": "ok", "id": args[0]}, nil
					}
					items, err := api.V2FirewallPolicies(cmd.Context(), slug)
					if err != nil {
						return nil, err
					}
					item, err := findLegacy(items, args[0])
					if err != nil {
						return nil, err
					}
					id := network.LegacyID(item)
					if err := api.DeleteV2FirewallPolicy(cmd.Context(), slug, id); err != nil {
						return nil, err
					}
					return map[string]any{"status": "ok", "id": id, "backend": "legacy-controller"}, nil
				})
			})
		},
	}
	forceFlag(del, &force)
	cmd.AddCommand(del)
	return cmd
}

func newPolicyLoggingCmd() *cobra.Command {
	var logging bool
	cmd := &cobra.Command{
		Use:   "logging <policy-id>",
		Short: "PATCH loggingEnabled on a firewall policy (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				return runMutation(cmd, "network firewall policies logging", fmt.Sprintf("Set loggingEnabled=%v on policy %s?", logging, args[0]), func() (any, error) {
					if isUUID(args[0]) {
						siteID, err := resolveSiteID(cmd.Context(), api, "")
						if err != nil {
							return nil, err
						}
						return api.PatchFirewallPolicy(cmd.Context(), siteID, args[0], map[string]any{"loggingEnabled": logging})
					}
					items, err := api.V2FirewallPolicies(cmd.Context(), slug)
					if err != nil {
						return nil, err
					}
					item, err := findLegacy(items, args[0])
					if err != nil {
						return nil, err
					}
					id := network.LegacyID(item)
					cur, err := api.V2FirewallPolicy(cmd.Context(), slug, id)
					if err != nil {
						cur = item
					}
					cur["logging"] = logging
					cur["loggingEnabled"] = logging
					return api.UpdateV2FirewallPolicy(cmd.Context(), slug, id, cur)
				})
			})
		},
	}
	cmd.Flags().BoolVar(&logging, "logging-enabled", true, "value for loggingEnabled")
	return cmd
}

func newPolicyEnableCmd(enable bool) *cobra.Command {
	use, short, action := "disable", "Disable a firewall policy (mutation)", "network firewall policies disable"
	if enable {
		use, short, action = "enable", "Enable a firewall policy (mutation)", "network firewall policies enable"
	}
	return &cobra.Command{
		Use:   use + " <policy-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				return runMutation(cmd, action, fmt.Sprintf("%s firewall policy %s?", use, args[0]), func() (any, error) {
					if isUUID(args[0]) {
						siteID, err := resolveSiteID(cmd.Context(), api, "")
						if err != nil {
							return nil, err
						}
						cur, err := api.FirewallPolicy(cmd.Context(), siteID, args[0])
						if err != nil {
							return nil, err
						}
						body := network.DropReadOnly(cur, "index")
						body["enabled"] = enable
						return api.UpdateFirewallPolicy(cmd.Context(), siteID, args[0], body)
					}
					items, err := api.V2FirewallPolicies(cmd.Context(), slug)
					if err != nil {
						return nil, err
					}
					item, err := findLegacy(items, args[0])
					if err != nil {
						return nil, err
					}
					id := network.LegacyID(item)
					cur, err := api.V2FirewallPolicy(cmd.Context(), slug, id)
					if err != nil {
						cur = item
					}
					cur["enabled"] = enable
					return api.UpdateV2FirewallPolicy(cmd.Context(), slug, id, cur)
				})
			})
		},
	}
}

func newNetworkACLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acl",
		Short: "ACL rules",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List ACL rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[network.ACLRule], error) {
					return api.ACLRules(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				en, err := parseEnabledFilter()
				if err != nil {
					return err
				}
				out := map[string]any{"siteId": siteID, "page": page}
				rows := make([][]string, 0, len(page.Data))
				for _, r := range page.Data {
					if !matchName(r.Name, rootOpts.filterName) {
						continue
					}
					if en != nil && r.Enabled != *en {
						continue
					}
					rows = append(rows, []string{itoa(r.Index), r.Name, yesNo(r.Enabled), r.ID})
				}
				return printList(cmd, out, []string{"INDEX", "NAME", "ENABLED", "ID"}, rows, page.Offset, len(rows), page.TotalCount)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <acl-id>",
		Short: "Get an ACL rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				r, err := api.ACLRule(cmd.Context(), siteID, args[0])
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, r, nil)
			})
		},
	})
	var createJSON string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create an ACL rule from JSON (mutation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(createJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network acl create", fmt.Sprintf("Create ACL %v?", body["name"]), func() (any, error) {
					return api.CreateACLRule(cmd.Context(), siteID, body)
				})
			})
		},
	}
	jsonFlag(create, &createJSON)
	_ = create.MarkFlagRequired("from-json")
	cmd.AddCommand(create)

	var updateJSON string
	update := &cobra.Command{
		Use:   "update <acl-id>",
		Short: "Update an ACL rule from JSON (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(updateJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network acl update", fmt.Sprintf("Update ACL %s?", args[0]), func() (any, error) {
					return api.UpdateACLRule(cmd.Context(), siteID, args[0], body)
				})
			})
		},
	}
	jsonFlag(update, &updateJSON)
	_ = update.MarkFlagRequired("from-json")
	cmd.AddCommand(update)

	var force bool
	del := &cobra.Command{
		Use:   "delete <acl-id>",
		Short: "Delete an ACL rule (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				return runMutation(cmd, "network acl delete", fmt.Sprintf("Delete ACL %s?", args[0]), func() (any, error) {
					if err := api.DeleteACLRule(cmd.Context(), siteID, args[0], force); err != nil {
						return nil, err
					}
					return map[string]any{"status": "ok", "id": args[0]}, nil
				})
			})
		},
	}
	forceFlag(del, &force)
	cmd.AddCommand(del)
	return cmd
}
