package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/access"
)

func newAccessCollectionCmd(use, short, collection string) *cobra.Command {
	cmd := &cobra.Command{Use: use, Short: short}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List " + use,
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openAccess()
			if err != nil {
				return err
			}
			items, err := api.Collection(cmd.Context(), collection)
			if err != nil {
				return mapAPIErr(err)
			}
			var filtered []map[string]any
			for _, item := range items {
				if matchName(access.ItemName(item), rootOpts.filterName) {
					filtered = append(filtered, item)
				}
			}
			total := len(filtered)
			start, end := sliceBounds(total)
			page := filtered[start:end]
			out := map[string]any{"count": len(page), "totalCount": total, use: page}
			rows := make([][]string, 0, len(page))
			for _, item := range page {
				rows = append(rows, []string{access.ItemName(item), anyString(item["type"]), anyString(item["status"]), access.ItemID(item)})
			}
			return printList(cmd, out, []string{"NAME", "TYPE", "STATUS", "ID"}, rows, start, len(page), total)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <id-or-name>",
		Short: "Get one " + use + " item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openAccess()
			if err != nil {
				return err
			}
			items, err := api.Collection(cmd.Context(), collection)
			if err != nil {
				return mapAPIErr(err)
			}
			id, err := resolveID(args[0], items, access.ItemName, access.ItemID)
			if err != nil {
				return err
			}
			item, err := api.Item(cmd.Context(), collection, id)
			if err != nil {
				for _, it := range items {
					if access.ItemID(it) == id {
						return printValue(cmd, it, nil)
					}
				}
				return mapAPIErr(err)
			}
			return printValue(cmd, item, nil)
		},
	})
	return cmd
}

func resolveAccessDoor(ctx context.Context, api *access.API, arg string) (string, error) {
	doors, err := api.Doors(ctx)
	if err != nil {
		return "", mapAPIErr(err)
	}
	return resolveID(arg, doors, doorDisplayName, func(d access.Door) string { return d.ID })
}

func resolveAccessUser(ctx context.Context, api *access.API, arg string) (string, error) {
	users, err := api.Users(ctx)
	if err != nil {
		return "", mapAPIErr(err)
	}
	return resolveID(arg, users, userDisplayName, func(u access.User) string { return u.ID })
}

func doorDisplayName(d access.Door) string {
	if d.Name != "" {
		return d.Name
	}
	return d.FullName
}

func userDisplayName(u access.User) string {
	if u.Name != "" {
		return u.Name
	}
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

func newAccessLockCmd(lock bool) *cobra.Command {
	use, short, action := "unlock", "Remote-unlock a door (mutation)", "UNLOCK"
	apiCall := func(ctx context.Context, api *access.API, id string) error {
		return api.UnlockDoor(ctx, id)
	}
	if lock {
		use, short, action = "lock", "Remote-lock a door (mutation)", "LOCK"
		apiCall = func(ctx context.Context, api *access.API, id string) error {
			return api.LockDoor(ctx, id)
		}
	}
	return &cobra.Command{
		Use:   use + " <door-id-or-name>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openAccess()
			if err != nil {
				return err
			}
			id, err := resolveAccessDoor(cmd.Context(), api, args[0])
			if err != nil {
				return err
			}
			return runMutation(cmd, "access doors "+use, fmt.Sprintf("%s door %s?", action, id), func() (any, error) {
				if err := apiCall(cmd.Context(), api, id); err != nil {
					return nil, err
				}
				return map[string]any{"action": action, "doorId": id, "status": "ok"}, nil
			})
		},
	}
}
