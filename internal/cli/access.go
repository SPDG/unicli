package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/access"
	"github.com/SPDG/unicli/internal/exitcode"
)

func newAccessCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "UniFi Access application commands",
		Long:  "Commands for UniFi Access. If Access is not installed on the console, unicli reports exit code unsupported (11).",
	}
	cmd.AddCommand(newAccessInfoCmd())
	cmd.AddCommand(newAccessDoorsCmd())
	cmd.AddCommand(newAccessUsersCmd())
	cmd.AddCommand(newAccessCollectionCmd("visitors", "Visitor passes", "visitors"))
	cmd.AddCommand(newAccessCollectionCmd("devices", "Access hubs, readers, and relays", "devices"))
	cmd.AddCommand(newAccessCollectionCmd("policies", "Access policies", "access_policies"))
	cmd.AddCommand(newAccessCollectionCmd("door-groups", "Door groups", "door_groups"))
	cmd.AddCommand(newAccessCollectionCmd("user-groups", "User groups", "user_groups"))
	return cmd
}

func openAccess() (*access.API, error) {
	res, err := resolveConnection()
	if err != nil {
		return nil, err
	}
	c, err := newHTTPClient(res)
	if err != nil {
		return nil, exitf(exitcode.Config, "%v", err)
	}
	return access.New(c), nil
}

func newAccessInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Check whether UniFi Access is available on this console",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openAccess()
			if err != nil {
				return err
			}
			info, err := api.Info(cmd.Context())
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, info, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "available=%v %s\n", info.Available, info.Message)
			})
		},
	}
}

func newAccessDoorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doors",
		Short: "Door commands",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List Access doors",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openAccess()
			if err != nil {
				return err
			}
			doors, err := api.Doors(cmd.Context())
			if err != nil {
				return mapAPIErr(err)
			}
			out := map[string]any{"count": len(doors), "doors": doors}
			rows := make([][]string, 0, len(doors))
			for _, d := range doors {
				name := d.Name
				if name == "" {
					name = d.FullName
				}
				locked := ""
				if d.IsLocked != nil {
					locked = fmt.Sprintf("%v", *d.IsLocked)
				} else if d.DoorLockRelayStatus != "" {
					locked = d.DoorLockRelayStatus
				}
				rows = append(rows, []string{name, locked, d.ID})
			}
			return printList(cmd, out, []string{"NAME", "LOCKED", "ID"}, rows, 0, len(doors), len(doors))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <door-id-or-name>",
		Short: "Get door details",
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
			door, err := api.Door(cmd.Context(), id)
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, door, nil)
		},
	})
	cmd.AddCommand(newAccessLockCmd(false))
	cmd.AddCommand(newAccessLockCmd(true))
	return cmd
}

func newAccessUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "User commands",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List Access users",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openAccess()
			if err != nil {
				return err
			}
			users, err := api.Users(cmd.Context())
			if err != nil {
				return mapAPIErr(err)
			}
			out := map[string]any{"count": len(users), "users": users}
			rows := make([][]string, 0, len(users))
			for _, u := range users {
				name := u.Name
				if name == "" {
					name = strings.TrimSpace(u.FirstName + " " + u.LastName)
				}
				rows = append(rows, []string{name, u.Email, u.ID})
			}
			return printList(cmd, out, []string{"NAME", "EMAIL", "ID"}, rows, 0, len(users), len(users))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <user-id-or-name>",
		Short: "Get Access user details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openAccess()
			if err != nil {
				return err
			}
			id, err := resolveAccessUser(cmd.Context(), api, args[0])
			if err != nil {
				return err
			}
			user, err := api.User(cmd.Context(), id)
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, user, nil)
		},
	})
	return cmd
}
