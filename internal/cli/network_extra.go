package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/network"
)

func listRawPage(cmd *cobra.Command, siteID string, page *network.Page[map[string]any], headers []string, keys []string) error {
	data := filterRaw(page.Data, "name")
	out := map[string]any{"siteId": siteID, "page": page}
	return printList(cmd, out, headers, rawRows(data, keys...), page.Offset, len(data), page.TotalCount)
}

func newNetworkDNSCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dns", Short: "DNS policies"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List DNS policies",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.DNSPolicies(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "TYPE", "ENABLED", "ID"}, []string{"name", "type", "enabled", "id"})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <dns-policy-id>",
		Short: "Get a DNS policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				n, err := api.DNSPolicy(cmd.Context(), siteID, args[0])
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, n, nil)
			})
		},
	})
	var createJSON, updateJSON string
	var force bool
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a DNS policy from JSON (mutation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(createJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network dns create", fmt.Sprintf("Create DNS policy type=%v?", body["type"]), func() (any, error) {
					return api.CreateDNSPolicy(cmd.Context(), siteID, body)
				})
			})
		},
	}
	jsonFlag(create, &createJSON)
	_ = create.MarkFlagRequired("from-json")
	cmd.AddCommand(create)
	update := &cobra.Command{
		Use:   "update <dns-policy-id>",
		Short: "Update a DNS policy from JSON (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(updateJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network dns update", fmt.Sprintf("Update DNS policy %s?", args[0]), func() (any, error) {
					return api.UpdateDNSPolicy(cmd.Context(), siteID, args[0], body)
				})
			})
		},
	}
	jsonFlag(update, &updateJSON)
	_ = update.MarkFlagRequired("from-json")
	cmd.AddCommand(update)
	del := &cobra.Command{
		Use:   "delete <dns-policy-id>",
		Short: "Delete a DNS policy (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				return runMutation(cmd, "network dns delete", fmt.Sprintf("Delete DNS policy %s?", args[0]), func() (any, error) {
					if err := api.DeleteDNSPolicy(cmd.Context(), siteID, args[0], force); err != nil {
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

func newNetworkVouchersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "vouchers", Short: "Hotspot vouchers"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List hotspot vouchers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.Vouchers(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "CODE", "ID"}, []string{"name", "code", "id"})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <voucher-id>",
		Short: "Get a voucher",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				n, err := api.Voucher(cmd.Context(), siteID, args[0])
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, n, nil)
			})
		},
	})
	var createJSON string
	var force bool
	create := &cobra.Command{
		Use:   "create",
		Short: "Generate hotspot vouchers from JSON (mutation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(createJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network vouchers create", fmt.Sprintf("Generate vouchers name=%v?", body["name"]), func() (any, error) {
					return api.CreateVouchers(cmd.Context(), siteID, body)
				})
			})
		},
	}
	jsonFlag(create, &createJSON)
	_ = create.MarkFlagRequired("from-json")
	cmd.AddCommand(create)
	del := &cobra.Command{
		Use:   "delete <voucher-id>",
		Short: "Delete a voucher (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				return runMutation(cmd, "network vouchers delete", fmt.Sprintf("Delete voucher %s?", args[0]), func() (any, error) {
					if err := api.DeleteVoucher(cmd.Context(), siteID, args[0], force); err != nil {
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

func newNetworkMatchingCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "matching-lists", Short: "Traffic matching lists"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List traffic matching lists",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.TrafficMatchingLists(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "TYPE", "ID"}, []string{"name", "type", "id"})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <list-id>",
		Short: "Get a traffic matching list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				n, err := api.TrafficMatchingList(cmd.Context(), siteID, args[0])
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, n, nil)
			})
		},
	})
	var createJSON, updateJSON string
	var force bool
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a traffic matching list from JSON (mutation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(createJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network matching-lists create", fmt.Sprintf("Create matching list %v?", body["name"]), func() (any, error) {
					return api.CreateTrafficMatchingList(cmd.Context(), siteID, body)
				})
			})
		},
	}
	jsonFlag(create, &createJSON)
	_ = create.MarkFlagRequired("from-json")
	cmd.AddCommand(create)
	update := &cobra.Command{
		Use:   "update <list-id>",
		Short: "Update a traffic matching list from JSON (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				body, err := readJSONBody(updateJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network matching-lists update", fmt.Sprintf("Update matching list %s?", args[0]), func() (any, error) {
					return api.UpdateTrafficMatchingList(cmd.Context(), siteID, args[0], body)
				})
			})
		},
	}
	jsonFlag(update, &updateJSON)
	_ = update.MarkFlagRequired("from-json")
	cmd.AddCommand(update)
	del := &cobra.Command{
		Use:   "delete <list-id>",
		Short: "Delete a traffic matching list (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				return runMutation(cmd, "network matching-lists delete", fmt.Sprintf("Delete matching list %s?", args[0]), func() (any, error) {
					if err := api.DeleteTrafficMatchingList(cmd.Context(), siteID, args[0], force); err != nil {
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

func newNetworkVPNCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "vpn", Short: "VPN servers and site-to-site tunnels"}
	cmd.AddCommand(&cobra.Command{
		Use:   "servers",
		Short: "List VPN servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.VPNServers(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "TYPE", "ID"}, []string{"name", "type", "id"})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "tunnels",
		Short: "List site-to-site VPN tunnels",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.VPNTunnels(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "ID"}, []string{"name", "id"})
			})
		},
	})
	return cmd
}

func newNetworkWANsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wans",
		Short: "List WAN interfaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.WANs(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "ID"}, []string{"name", "id"})
			})
		},
	}
}

func newNetworkDPICmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dpi", Short: "DPI applications and categories"}
	cmd.AddCommand(&cobra.Command{
		Use:   "apps",
		Short: "List DPI applications",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, _, err := openNetwork()
			if err != nil {
				return err
			}
			page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
				return api.DPIApplications(cmd.Context(), offset, limit)
			})
			if err != nil {
				return mapAPIErr(err)
			}
			data := filterRaw(page.Data, "name")
			return printList(cmd, page, []string{"NAME", "ID"}, rawRows(data, "name", "id"), page.Offset, len(data), page.TotalCount)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "categories",
		Short: "List DPI categories",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, _, err := openNetwork()
			if err != nil {
				return err
			}
			page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
				return api.DPICategories(cmd.Context(), offset, limit)
			})
			if err != nil {
				return mapAPIErr(err)
			}
			data := filterRaw(page.Data, "name")
			return printList(cmd, page, []string{"NAME", "ID"}, rawRows(data, "name", "id"), page.Offset, len(data), page.TotalCount)
		},
	})
	return cmd
}

func newNetworkRadiusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "radius",
		Short: "List RADIUS profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.RadiusProfiles(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "ID"}, []string{"name", "id"})
			})
		},
	}
}

func newNetworkSwitchingCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "switching", Short: "Switch stacks, LAGs, MC-LAG"}
	cmd.AddCommand(&cobra.Command{
		Use:   "stacks",
		Short: "List switch stacks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.SwitchStacks(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "ID"}, []string{"name", "id"})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "stack <id>",
		Short: "Get a switch stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				n, err := api.SwitchStack(cmd.Context(), siteID, args[0])
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, n, nil)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "lags",
		Short: "List LAGs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.LAGs(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "ID"}, []string{"name", "id"})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "lag <id>",
		Short: "Get a LAG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				n, err := api.LAG(cmd.Context(), siteID, args[0])
				if err != nil {
					return mapAPIErr(err)
				}
				return printValue(cmd, n, nil)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "mclag",
		Short: "List MC-LAG domains",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.MCLAGDomains(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "ID"}, []string{"name", "id"})
			})
		},
	})
	return cmd
}

func newNetworkTagsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List device tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
					return api.DeviceTags(cmd.Context(), siteID, offset, limit)
				})
				if err != nil {
					return mapAPIErr(err)
				}
				return listRawPage(cmd, siteID, page, []string{"NAME", "ID"}, []string{"name", "id"})
			})
		},
	}
}

func newNetworkPendingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pending-devices",
		Short: "List devices pending adoption",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, _, err := openNetwork()
			if err != nil {
				return err
			}
			page, err := collectPage(func(offset, limit int) (*network.Page[map[string]any], error) {
				return api.PendingDevices(cmd.Context(), offset, limit)
			})
			if err != nil {
				return mapAPIErr(err)
			}
			data := filterRaw(page.Data, "name")
			return printList(cmd, page, []string{"NAME", "MAC", "ID"}, rawRows(data, "name", "macAddress", "id"), page.Offset, len(data), page.TotalCount)
		},
	}
}
