package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/client"
	"github.com/SPDG/unicli/internal/console"
	"github.com/SPDG/unicli/internal/exitcode"
)

func newConsoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console",
		Short: "UniFi OS console (hardware, apps, firmware, reboot)",
	}
	cmd.AddCommand(newConsoleStatusCmd())
	cmd.AddCommand(newConsoleUpdatesCmd())
	cmd.AddCommand(newConsoleRebootCmd())
	return cmd
}

func openConsole() (*console.API, error) {
	res, err := resolveConnection()
	if err != nil {
		return nil, err
	}
	c, err := newHTTPClient(res)
	if err != nil {
		return nil, exitf(exitcode.Config, "%v", err)
	}
	return console.New(c), nil
}

func newConsoleStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Console hardware plus Network/Protect/Access availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openConsole()
			if err != nil {
				return err
			}
			sys, err := api.System(cmd.Context())
			if err != nil {
				return mapAPIErr(err)
			}
			osApps, _ := api.OSApps(cmd.Context())
			apps := api.ProbeApps(cmd.Context())
			out := map[string]any{"system": sys, "apps": apps}
			if osApps != nil {
				out["osApps"] = osApps
			}
			return printValue(cmd, out, func() {
				short := ""
				if hw, ok := sys["hardware"].(map[string]any); ok {
					short = anyString(hw["shortname"])
				}
				if short == "" {
					short = anyString(sys["hardware"])
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s state=%s internet=%v cloud=%v\n",
					anyString(sys["name"]), short, anyString(sys["deviceState"]),
					sys["hasInternet"], sys["cloudConnected"])
				for _, a := range apps {
					if a.Available {
						fmt.Fprintf(cmd.OutOrStdout(), "  %-8s %s\n", a.Name, a.Version)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  %-8s unavailable\n", a.Name)
					}
				}
			})
		},
	}
}

func newConsoleUpdatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "updates",
		Short: "UniFi OS firmware update status (if the API key is allowed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openConsole()
			if err != nil {
				return err
			}
			fw, err := api.FirmwareUpdate(cmd.Context())
			if err != nil {
				var ae client.APIError
				if errors.As(err, &ae) && (ae.Status == 401 || ae.Status == 403) {
					return exitf(exitcode.Permission, "this API key cannot read UniFi OS firmware updates (HTTP %d)", ae.Status)
				}
				return mapAPIErr(err)
			}
			return printValue(cmd, fw, nil)
		},
	}
}

func newConsoleRebootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reboot",
		Short: "Reboot the UniFi OS console (mutation)",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openConsole()
			if err != nil {
				return err
			}
			return runMutation(cmd, "console reboot", "Reboot the UniFi OS console?", func() (any, error) {
				if err := api.Reboot(cmd.Context()); err != nil {
					return nil, err
				}
				return map[string]any{"action": "REBOOT", "status": "ok"}, nil
			})
		},
	}
}
