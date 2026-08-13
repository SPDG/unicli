package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
)

func newNetworkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "UniFi Network application commands",
	}
	cmd.AddCommand(newNetworkInfoCmd())
	cmd.AddCommand(newNetworkSitesCmd())
	cmd.AddCommand(newNetworkDevicesCmd())
	cmd.AddCommand(newNetworkPortsCmd())
	cmd.AddCommand(newNetworkClientsCmd())
	cmd.AddCommand(newNetworkNetworksCmd())
	cmd.AddCommand(newNetworkWifiCmd())
	cmd.AddCommand(newNetworkFirewallCmd())
	cmd.AddCommand(newNetworkACLCmd())
	cmd.AddCommand(newNetworkDNSCmd())
	cmd.AddCommand(newNetworkVouchersCmd())
	cmd.AddCommand(newNetworkMatchingCmd())
	cmd.AddCommand(newNetworkVPNCmd())
	cmd.AddCommand(newNetworkWANsCmd())
	cmd.AddCommand(newNetworkDPICmd())
	cmd.AddCommand(newNetworkRadiusCmd())
	cmd.AddCommand(newNetworkSwitchingCmd())
	cmd.AddCommand(newNetworkTagsCmd())
	cmd.AddCommand(newNetworkPendingCmd())
	cmd.AddCommand(newNetworkRoutesCmd())
	cmd.AddCommand(newNetworkTrafficRoutesCmd())
	cmd.AddCommand(newNetworkPortProfilesCmd())
	cmd.AddCommand(newNetworkListsCmd())
	cmd.AddCommand(newNetworkPortForwardsCmd())
	cmd.AddCommand(newNetworkDHCPCmd())
	cmd.AddCommand(newNetworkHealthCmd())
	cmd.AddCommand(newNetworkSysinfoCmd())
	cmd.AddCommand(newNetworkDynamicDNSCmd())
	cmd.AddCommand(newNetworkClientGroupsCmd())
	return cmd
}

func withLegacySite(cmd *cobra.Command, fn func(api *network.API, slug string) error) error {
	api, preferredSite, err := openNetwork()
	if err != nil {
		return err
	}
	slug, err := api.LegacySiteSlug(cmd.Context(), preferredSite)
	if err != nil {
		return mapAPIErr(err)
	}
	return fn(api, slug)
}

func withNetworkSite(cmd *cobra.Command, fn func(api *network.API, siteID string) error) error {
	api, preferredSite, err := openNetwork()
	if err != nil {
		return err
	}
	siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
	if err != nil {
		return mapAPIErr(err)
	}
	return fn(api, siteID)
}

func openNetwork() (*network.API, string, error) {
	res, err := resolveConnection()
	if err != nil {
		return nil, "", err
	}
	c, err := newHTTPClient(res)
	if err != nil {
		return nil, "", exitf(exitcode.Config, "%v", err)
	}
	api := network.New(c)
	return api, res.Site, nil
}

func newNetworkInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show Network application info",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, _, err := openNetwork()
			if err != nil {
				return err
			}
			info, err := api.Info(cmd.Context())
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, info, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "applicationVersion=%s\n", info.ApplicationVersion)
			})
		},
	}
}

func newNetworkSitesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Site commands",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List Network sites",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, _, err := openNetwork()
			if err != nil {
				return err
			}
			page, err := collectPage(func(offset, limit int) (*network.Page[network.Site], error) {
				return api.Sites(cmd.Context(), offset, limit)
			})
			if err != nil {
				return mapAPIErr(err)
			}
			rows := make([][]string, 0, len(page.Data))
			for _, s := range page.Data {
				if !matchName(s.Name, rootOpts.filterName) && !matchName(s.InternalRef, rootOpts.filterName) {
					continue
				}
				rows = append(rows, []string{s.Name, s.InternalRef, s.ID})
			}
			return printList(cmd, page, []string{"NAME", "REF", "ID"}, rows, page.Offset, len(rows), page.TotalCount)
		},
	})
	return cmd
}

func newNetworkDevicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Device commands",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List adopted devices",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			page, err := collectPage(func(offset, limit int) (*network.Page[network.Device], error) {
				return api.Devices(cmd.Context(), siteID, offset, limit)
			})
			if err != nil {
				return mapAPIErr(err)
			}
			out := map[string]any{"siteId": siteID, "page": page}
			rows := make([][]string, 0, len(page.Data))
			for _, d := range page.Data {
				if !matchName(d.Name, rootOpts.filterName) && !matchName(d.Model, rootOpts.filterName) {
					continue
				}
				rows = append(rows, []string{d.Name, d.Model, d.IPAddress, d.State, d.ID})
			}
			return printList(cmd, out, []string{"NAME", "MODEL", "IP", "STATE", "ID"}, rows, page.Offset, len(rows), page.TotalCount)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <device-id>",
		Short: "Get device details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			dev, err := api.Device(cmd.Context(), siteID, args[0])
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, dev, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s %s\n", dev.ID, dev.Name, dev.Model, dev.State)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "stats <device-id>",
		Short: "Get latest device statistics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			stats, err := api.DeviceStatistics(cmd.Context(), siteID, args[0])
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, stats, nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "restart <device-id>",
		Short: "Restart a device (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireMutations("network devices restart"); err != nil {
				return err
			}
			if err := requireConfirm(fmt.Sprintf("Restart device %s?", args[0])); err != nil {
				return err
			}
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			if err := api.RestartDevice(cmd.Context(), siteID, args[0]); err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, map[string]any{
				"action": "RESTART", "deviceId": args[0], "siteId": siteID, "status": "ok",
			}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "restart requested for %s\n", args[0])
			})
		},
	})
	return cmd
}

func newNetworkPortsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ports",
		Short: "Switch port commands",
	}
	cmd.AddCommand(newPortsListCmd())
	cmd.AddCommand(newPortsSetCmd())
	cmd.AddCommand(&cobra.Command{
		Use:   "cycle <device-id> <port-idx>",
		Short: "PoE power-cycle a switch port (mutation)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireMutations("network ports cycle"); err != nil {
				return err
			}
			portIdx, err := strconv.Atoi(args[1])
			if err != nil || portIdx < 0 {
				return exitf(exitcode.Usage, "invalid port index %q", args[1])
			}
			if err := requireConfirm(fmt.Sprintf("Power-cycle port %d on device %s?", portIdx, args[0])); err != nil {
				return err
			}
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			if err := api.PowerCyclePort(cmd.Context(), siteID, args[0], portIdx); err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, map[string]any{
				"action": "POWER_CYCLE", "deviceId": args[0], "portIdx": portIdx, "siteId": siteID, "status": "ok",
			}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "power-cycle requested for %s port %d\n", args[0], portIdx)
			})
		},
	})
	return cmd
}

func newNetworkClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Client commands",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List connected clients",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			page, err := collectPage(func(offset, limit int) (*network.Page[network.Client], error) {
				return api.Clients(cmd.Context(), siteID, offset, limit)
			})
			if err != nil {
				return mapAPIErr(err)
			}
			out := map[string]any{"siteId": siteID, "page": page}
			rows := make([][]string, 0, len(page.Data))
			for _, c := range page.Data {
				if !matchName(c.Name, rootOpts.filterName) && !matchName(c.IPAddress, rootOpts.filterName) && !matchName(c.MacAddress, rootOpts.filterName) {
					continue
				}
				rows = append(rows, []string{c.Name, c.IPAddress, c.MacAddress, c.Type, c.ID})
			}
			return printList(cmd, out, []string{"NAME", "IP", "MAC", "TYPE", "ID"}, rows, page.Offset, len(rows), page.TotalCount)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <client-id>",
		Short: "Get client details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			cl, err := api.Client(cmd.Context(), siteID, args[0])
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, cl, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", cl.ID, cl.Name, cl.IPAddress)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "authorize <client-id>",
		Short: "Authorize guest access for a client (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireMutations("network clients authorize"); err != nil {
				return err
			}
			if err := requireConfirm(fmt.Sprintf("Authorize guest access for client %s?", args[0])); err != nil {
				return err
			}
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			if err := api.AuthorizeGuest(cmd.Context(), siteID, args[0]); err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, map[string]any{
				"action": "AUTHORIZE_GUEST_ACCESS", "clientId": args[0], "siteId": siteID, "status": "ok",
			}, nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "unauthorize <client-id>",
		Short: "Unauthorize guest access for a client (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireMutations("network clients unauthorize"); err != nil {
				return err
			}
			if err := requireConfirm(fmt.Sprintf("Unauthorize guest access for client %s?", args[0])); err != nil {
				return err
			}
			api, preferredSite, err := openNetwork()
			if err != nil {
				return err
			}
			siteID, err := resolveSiteID(cmd.Context(), api, preferredSite)
			if err != nil {
				return mapAPIErr(err)
			}
			if err := api.UnauthorizeGuest(cmd.Context(), siteID, args[0]); err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, map[string]any{
				"action": "UNAUTHORIZE_GUEST_ACCESS", "clientId": args[0], "siteId": siteID, "status": "ok",
			}, nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "kick <client-id>",
		Short: "Disconnect a client (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withNetworkSite(cmd, func(api *network.API, siteID string) error {
				return runMutation(cmd, "network clients kick", fmt.Sprintf("Kick client %s?", args[0]), func() (any, error) {
					if err := api.KickClient(cmd.Context(), siteID, args[0]); err != nil {
						return nil, err
					}
					return map[string]any{"action": "KICK", "clientId": args[0], "siteId": siteID, "status": "ok"}, nil
				})
			})
		},
	})
	return cmd
}
