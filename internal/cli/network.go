package cli

import (
	"fmt"

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
	return cmd
}
