package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
)

func legacyWrap(resource, slug string, data any) map[string]any {
	return map[string]any{
		"backend":  "legacy-controller",
		"resource": resource,
		"site":     slug,
		"data":     data,
	}
}

func findLegacy(items []map[string]any, arg string) (map[string]any, error) {
	arg = strings.TrimSpace(arg)
	var exact, partial []map[string]any
	for _, item := range items {
		id := network.LegacyID(item)
		name := anyString(item["name"])
		if id == arg || name == arg {
			exact = append(exact, item)
			continue
		}
		if matchName(name, arg) {
			partial = append(partial, item)
		}
	}
	hits := exact
	if len(hits) == 0 {
		hits = partial
	}
	if len(hits) == 0 {
		return nil, exitf(exitcode.NotFound, "no match for %q", arg)
	}
	if len(hits) > 1 {
		return nil, exitf(exitcode.Usage, "ambiguous %q (%d matches)", arg, len(hits))
	}
	return hits[0], nil
}

func newNetworkRoutesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "Static routes",
		Long:  "Static routes via /proxy/network/api/s/{site}/rest/routing. Same API key as Integration; site slug is internalReference (usually default). Will move to Integration when Ubiquiti ships it.",
	}
	cmd.AddCommand(legacyListCmd("list", "List static routes", "routing", []string{"NAME", "NETWORK", "NEXTHOP", "DISTANCE", "ENABLED", "ID"},
		[]string{"name", "static-route_network", "static-route_nexthop", "static-route_distance", "enabled", "_id"}))
	cmd.AddCommand(legacyGetCmd("get <route-id-or-name>", "Get a static route", "routing"))
	cmd.AddCommand(legacyCreateCmd("create", "Create a static route from JSON (mutation)", "network routes create", "routing"))
	cmd.AddCommand(legacyUpdateCmd("update <route-id-or-name>", "Update a static route from JSON (mutation)", "network routes update", "routing"))
	cmd.AddCommand(legacyDeleteCmd("delete <route-id-or-name>", "Delete a static route (mutation)", "network routes delete", "routing"))
	return cmd
}

func newNetworkTrafficRoutesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traffic-routes",
		Short: "Policy-based traffic routes",
		Long:  "Policy-based traffic routes (WAN / VPN steering).",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List traffic routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.TrafficRoutes(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				page := network.SlicePage(filterRaw(items, "name"), rootOpts.offset, rootOpts.limit, rootOpts.allPages)
				out := legacyWrap("traffic-routes", slug, page)
				return printList(cmd, out, []string{"NAME", "ENABLED", "ID"}, rawRows(page.Data, "name", "enabled", "_id"), page.Offset, len(page.Data), page.TotalCount)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <route-id-or-name>",
		Short: "Get a traffic route",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.TrafficRoutes(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				item, err := findLegacy(items, args[0])
				if err != nil {
					return err
				}
				return printValue(cmd, legacyWrap("traffic-routes", slug, item), nil)
			})
		},
	})
	var createJSON, updateJSON string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a traffic route from JSON (mutation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				body, err := readJSONBody(createJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, "network traffic-routes create", fmt.Sprintf("Create traffic route %v?", body["name"]), func() (any, error) {
					out, err := api.CreateTrafficRoute(cmd.Context(), slug, body)
					if err != nil {
						return nil, err
					}
					return legacyWrap("traffic-routes", slug, out), nil
				})
			})
		},
	}
	jsonFlag(create, &createJSON)
	_ = create.MarkFlagRequired("from-json")
	cmd.AddCommand(create)
	update := &cobra.Command{
		Use:   "update <route-id-or-name>",
		Short: "Update a traffic route from JSON (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				body, err := readJSONBody(updateJSON)
				if err != nil {
					return err
				}
				items, err := api.TrafficRoutes(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				cur, err := findLegacy(items, args[0])
				if err != nil {
					return err
				}
				id := network.LegacyID(cur)
				return runMutation(cmd, "network traffic-routes update", fmt.Sprintf("Update traffic route %s?", args[0]), func() (any, error) {
					out, err := api.UpdateTrafficRoute(cmd.Context(), slug, id, body)
					if err != nil {
						return nil, err
					}
					return legacyWrap("traffic-routes", slug, out), nil
				})
			})
		},
	}
	jsonFlag(update, &updateJSON)
	_ = update.MarkFlagRequired("from-json")
	cmd.AddCommand(update)
	cmd.AddCommand(&cobra.Command{
		Use:   "delete <route-id-or-name>",
		Short: "Delete a traffic route (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.TrafficRoutes(cmd.Context(), slug)
				if err != nil {
					return mapAPIErr(err)
				}
				cur, err := findLegacy(items, args[0])
				if err != nil {
					return err
				}
				id := network.LegacyID(cur)
				return runMutation(cmd, "network traffic-routes delete", fmt.Sprintf("Delete traffic route %s?", args[0]), func() (any, error) {
					if err := api.DeleteTrafficRoute(cmd.Context(), slug, id); err != nil {
						return nil, err
					}
					return legacyWrap("traffic-routes", slug, map[string]any{"status": "ok", "_id": id}), nil
				})
			})
		},
	})
	return cmd
}

func newNetworkPortProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-profiles",
		Short: "Switch port profiles",
		Long:  "Port profiles via /proxy/network/api/s/{site}/rest/portconf. Same API key; site slug is internalReference.",
	}
	cmd.AddCommand(legacyListCmd("list", "List port profiles", "portconf",
		[]string{"NAME", "FORWARD", "POE", "ID"},
		[]string{"name", "forward", "poe_mode", "_id"}))
	cmd.AddCommand(legacyGetCmd("get <profile-id-or-name>", "Get a port profile", "portconf"))
	cmd.AddCommand(legacyCreateCmd("create", "Create a port profile from JSON (mutation)", "network port-profiles create", "portconf"))
	cmd.AddCommand(legacyUpdateCmd("update <profile-id-or-name>", "Update a port profile from JSON (mutation)", "network port-profiles update", "portconf"))
	cmd.AddCommand(legacyDeleteCmd("delete <profile-id-or-name>", "Delete a port profile (mutation)", "network port-profiles delete", "portconf"))
	return cmd
}

func newNetworkListsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lists",
		Aliases: []string{"groups"},
		Short:   "Firewall address/port groups",
		Long:    "Classic firewall groups via rest/firewallgroup. For Policy Engine traffic-matching lists use `network matching-lists`.",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List firewall groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.LegacyList(cmd.Context(), slug, "firewallgroup")
				if err != nil {
					return mapAPIErr(err)
				}
				items = filterRaw(items, "name")
				page := network.SlicePage(items, rootOpts.offset, rootOpts.limit, rootOpts.allPages)
				rows := make([][]string, 0, len(page.Data))
				for _, item := range page.Data {
					members := item["group_members"]
					n := 0
					if arr, ok := members.([]any); ok {
						n = len(arr)
					}
					rows = append(rows, []string{
						anyString(item["name"]),
						anyString(item["group_type"]),
						strconv.Itoa(n),
						network.LegacyID(item),
					})
				}
				out := legacyWrap("firewallgroup", slug, page)
				return printList(cmd, out, []string{"NAME", "TYPE", "MEMBERS", "ID"}, rows, page.Offset, len(page.Data), page.TotalCount)
			})
		},
	})
	cmd.AddCommand(legacyGetCmd("get <list-id-or-name>", "Get a firewall group", "firewallgroup"))
	cmd.AddCommand(legacyCreateCmd("create", "Create a firewall group from JSON (mutation)", "network lists create", "firewallgroup"))
	cmd.AddCommand(legacyUpdateCmd("update <list-id-or-name>", "Update a firewall group from JSON (mutation)", "network lists update", "firewallgroup"))
	cmd.AddCommand(legacyDeleteCmd("delete <list-id-or-name>", "Delete a firewall group (mutation)", "network lists delete", "firewallgroup"))
	return cmd
}

func legacyListCmd(use, short, collection string, headers, keys []string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.LegacyList(cmd.Context(), slug, collection)
				if err != nil {
					return mapAPIErr(err)
				}
				page := network.SlicePage(filterRaw(items, "name"), rootOpts.offset, rootOpts.limit, rootOpts.allPages)
				out := legacyWrap(collection, slug, page)
				return printList(cmd, out, headers, rawRows(page.Data, keys...), page.Offset, len(page.Data), page.TotalCount)
			})
		},
	}
}

func legacyGetCmd(use, short, collection string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.LegacyList(cmd.Context(), slug, collection)
				if err != nil {
					return mapAPIErr(err)
				}
				item, err := findLegacy(items, args[0])
				if err != nil {
					return err
				}
				return printValue(cmd, legacyWrap(collection, slug, item), nil)
			})
		},
	}
}

func legacyCreateCmd(use, short, action, collection string) *cobra.Command {
	var fromJSON string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				body, err := readJSONBody(fromJSON)
				if err != nil {
					return err
				}
				return runMutation(cmd, action, fmt.Sprintf("Create %s %v?", collection, body["name"]), func() (any, error) {
					out, err := api.LegacyCreate(cmd.Context(), slug, collection, body)
					if err != nil {
						return nil, err
					}
					return legacyWrap(collection, slug, out), nil
				})
			})
		},
	}
	jsonFlag(cmd, &fromJSON)
	_ = cmd.MarkFlagRequired("from-json")
	return cmd
}

func legacyUpdateCmd(use, short, action, collection string) *cobra.Command {
	var fromJSON string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				body, err := readJSONBody(fromJSON)
				if err != nil {
					return err
				}
				items, err := api.LegacyList(cmd.Context(), slug, collection)
				if err != nil {
					return mapAPIErr(err)
				}
				cur, err := findLegacy(items, args[0])
				if err != nil {
					return err
				}
				id := network.LegacyID(cur)
				return runMutation(cmd, action, fmt.Sprintf("Update %s %s?", collection, args[0]), func() (any, error) {
					out, err := api.LegacyUpdate(cmd.Context(), slug, collection, id, body)
					if err != nil {
						return nil, err
					}
					return legacyWrap(collection, slug, out), nil
				})
			})
		},
	}
	jsonFlag(cmd, &fromJSON)
	_ = cmd.MarkFlagRequired("from-json")
	return cmd
}

func legacyDeleteCmd(use, short, action, collection string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withLegacySite(cmd, func(api *network.API, slug string) error {
				items, err := api.LegacyList(cmd.Context(), slug, collection)
				if err != nil {
					return mapAPIErr(err)
				}
				cur, err := findLegacy(items, args[0])
				if err != nil {
					return err
				}
				id := network.LegacyID(cur)
				return runMutation(cmd, action, fmt.Sprintf("Delete %s %s?", collection, args[0]), func() (any, error) {
					if err := api.LegacyDelete(cmd.Context(), slug, collection, id); err != nil {
						return nil, err
					}
					return legacyWrap(collection, slug, map[string]any{"status": "ok", "_id": id}), nil
				})
			})
		},
	}
}
