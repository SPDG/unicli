package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
)

func newPortsFindCmd() *cobra.Command {
	var mac, ip, client string
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Find switch ports by client MAC, IP, or name",
		RunE: func(cmd *cobra.Command, args []string) error {
			if mac == "" && ip == "" && client == "" && rootOpts.filterName == "" {
				return exitf(exitcode.Usage, "need --mac, --ip, --client, or --name")
			}
			name := client
			if name == "" {
				name = rootOpts.filterName
			}
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				devs, err := api.StatDevices(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				clients, err := api.ActiveClients(cmd.Context(), slug)
				if err != nil {
					clients, _ = api.StatSta(cmd.Context(), slug)
				}
				found := network.FindPorts(devs, clients, mac, ip, name)
				if len(found) == 0 {
					return exitf(exitcode.NotFound, "no matching ports")
				}
				paged := network.SlicePage(network.PortViewsAsMaps(found), rootOpts.offset, rootOpts.limit, true)
				rows := make([][]string, 0, len(found))
				for _, v := range found {
					rows = append(rows, []string{v.Device, fmt.Sprintf("%d", v.Port), v.Name, anyString(v.SpeedMbps), strings.Join(v.Clients, ",")})
				}
				return printList(cmd, map[string]any{"backend": "merged", "resource": "device-ports", "site": slug, "data": paged},
					[]string{"DEVICE", "PORT", "NAME", "SPEED", "CLIENTS"},
					rows, 0, len(rows), len(rows))
			})
		},
	}
	cmd.Flags().StringVar(&mac, "mac", "", "client or device MAC")
	cmd.Flags().StringVar(&ip, "ip", "", "client IP")
	cmd.Flags().StringVar(&client, "client", "", "client name substring")
	return cmd
}

func newNetworkTopologyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topology",
		Short: "L2/L3 topology from the Network v2 graph",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "path <client|mac|ip> <client|mac|ip>",
		Short: "Trace the switch/AP path between two clients or devices",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDiagSite(cmd, func(api *network.API, siteID, slug string) error {
				from, err := api.ResolveMAC(cmd.Context(), siteID, slug, args[0])
				if err != nil {
					return exitf(exitcode.NotFound, "%v", err)
				}
				to, err := api.ResolveMAC(cmd.Context(), siteID, slug, args[1])
				if err != nil {
					return exitf(exitcode.NotFound, "%v", err)
				}
				topo, err := api.Topology(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				devs, _ := api.StatDevices(cmd.Context(), slug)
				tr := network.TracePath(topo, devs, from.MacAddress, to.MacAddress)
				tr.From, tr.To = from, to
				if !tr.Found {
					return exitf(exitcode.NotFound, "no topology path between %s and %s", args[0], args[1])
				}
				return printValue(cmd, tr, func() {
					if !tr.Complete {
						for _, n := range tr.Notes {
							fmt.Fprintln(cmd.OutOrStdout(), n)
						}
					}
					for i, hop := range tr.Hops {
						link := ""
						if hop.Uplink != nil {
							link = fmt.Sprintf(" <- %s p%s %s", hop.Uplink.Device, anyString(derefInt(hop.Uplink.Port)), anyString(derefInt(hop.Uplink.RateMbps)))
						}
						extra := ""
						if hop.Attachment != "" && hop.Attachment != network.AttachmentPhysical {
							extra = " [" + hop.Attachment + "]"
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%d %s %s %s%s%s\n", i, hop.Kind, hop.Name, hop.MAC, link, extra)
					}
				})
			})
		},
	})
	return cmd
}

func newNetworkDiagnoseCmd() *cobra.Command {
	var client string
	cmd := &cobra.Command{
		Use:   "diagnose",
		Short: "Compact client diagnostic: health, access port, uplink path",
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == "" && len(args) == 1 {
				client = args[0]
			}
			if client == "" {
				return exitf(exitcode.Usage, "need --client <name|mac|ip>")
			}
			return withDiagSite(cmd, func(api *network.API, siteID, slug string) error {
				view, err := api.LookupClientView(cmd.Context(), siteID, slug, client)
				if err != nil {
					return exitf(exitcode.NotFound, "%v", err)
				}
				healthItems, herr := api.StatHealth(cmd.Context(), slug)
				if herr != nil {
					return mapAPIErr(herr)
				}
				vpnServers, vpnTunnels := 0, 0
				if page, err := api.VPNServers(cmd.Context(), siteID, 0, 1); err == nil {
					vpnServers = page.TotalCount
				}
				if page, err := api.VPNTunnels(cmd.Context(), siteID, 0, 1); err == nil {
					vpnTunnels = page.TotalCount
				}
				topo, _ := api.Topology(cmd.Context(), slug)
				devs, _ := api.StatDevices(cmd.Context(), slug)
				rep := network.DiagnoseReport{
					Client:     view,
					UplinkPath: network.UplinkChain(topo, devs, view.MacAddress),
					Health:     network.AnnotateHealth(healthItems, vpnServers, vpnTunnels),
				}
				if view.Uplink != nil && view.Uplink.Port != nil {
					if d := findDeviceMAC(devs, view.Uplink.DeviceMAC); d != nil {
						raw, _ := d["port_table"].([]any)
						for _, item := range raw {
							p, ok := item.(map[string]any)
							if !ok {
								continue
							}
							if network.CompactPort(d, p).Port == *view.Uplink.Port {
								pv := network.CompactPort(d, p)
								rep.AccessPort = &pv
								break
							}
						}
					}
				}
				return printValue(cmd, rep, func() {
					fmt.Fprintf(cmd.OutOrStdout(), "%s %s vlan=%s %s\n", view.Name, view.IPAddress, anyString(view.VLAN), view.MacAddress)
					if rep.AccessPort != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "port %s %d %s speed=%s down=%s\n",
							rep.AccessPort.Device, rep.AccessPort.Port, rep.AccessPort.Name,
							anyString(rep.AccessPort.SpeedMbps), anyString(rep.AccessPort.LinkDownCount))
					}
				})
			})
		},
	}
	cmd.Flags().StringVar(&client, "client", "", "client name, MAC, or IP")
	return cmd
}

func derefInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func findDeviceMAC(devs []map[string]any, mac string) map[string]any {
	want := network.NormalizeMAC(mac)
	for _, d := range devs {
		if network.NormalizeMAC(anyString(d["mac"])) == want {
			return d
		}
	}
	return nil
}
