package cli

import (
	"fmt"

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
			return printValue(cmd, out, func() {
				for _, d := range doors {
					name := d.Name
					if name == "" {
						name = d.FullName
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", d.ID, name)
				}
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <door-id>",
		Short: "Get door details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openAccess()
			if err != nil {
				return err
			}
			door, err := api.Door(cmd.Context(), args[0])
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, door, nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "unlock <door-id>",
		Short: "Remote-unlock a door (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireMutations("access doors unlock"); err != nil {
				return err
			}
			if err := requireConfirm(fmt.Sprintf("Unlock door %s?", args[0])); err != nil {
				return err
			}
			api, err := openAccess()
			if err != nil {
				return err
			}
			if err := api.UnlockDoor(cmd.Context(), args[0]); err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, map[string]any{
				"action": "UNLOCK", "doorId": args[0], "status": "ok",
			}, nil)
		},
	})
	return cmd
}
