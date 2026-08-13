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
	return cmd
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
			page, err := api.Sites(cmd.Context(), rootOpts.offset, rootOpts.limit)
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, page, func() {
				for _, s := range page.Data {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", s.ID, s.InternalRef, s.Name)
				}
			})
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
			page, err := api.Devices(cmd.Context(), siteID, rootOpts.offset, rootOpts.limit)
			if err != nil {
				return mapAPIErr(err)
			}
			out := map[string]any{"siteId": siteID, "page": page}
			return printValue(cmd, out, func() {
				for _, d := range page.Data {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", d.ID, d.Name, d.Model, d.IPAddress, d.State)
				}
			})
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
			page, err := api.Clients(cmd.Context(), siteID, rootOpts.offset, rootOpts.limit)
			if err != nil {
				return mapAPIErr(err)
			}
			out := map[string]any{"siteId": siteID, "page": page}
			return printValue(cmd, out, func() {
				for _, c := range page.Data {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", c.ID, c.Name, c.IPAddress, c.MacAddress)
				}
			})
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
	return cmd
}
