package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/protect"
)

func newProtectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protect",
		Short: "UniFi Protect application commands",
	}
	cmd.AddCommand(newProtectInfoCmd())
	cmd.AddCommand(newProtectNVRCmd())
	cmd.AddCommand(newProtectCamerasCmd())
	cmd.AddCommand(newProtectLiveviewsCmd())
	cmd.AddCommand(newProtectDevicesCmd("lights", "Flood / AI lights"))
	cmd.AddCommand(newProtectDevicesCmd("sensors", "Protect sensors"))
	cmd.AddCommand(newProtectDevicesCmd("chimes", "Doorbell chimes"))
	cmd.AddCommand(newProtectDevicesCmd("viewers", "Viewports / viewers"))
	return cmd
}

func openProtect() (*protect.API, error) {
	res, err := resolveConnection()
	if err != nil {
		return nil, err
	}
	c, err := newHTTPClient(res)
	if err != nil {
		return nil, exitf(exitcode.Config, "%v", err)
	}
	return protect.New(c), nil
}

func newProtectInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show Protect application info",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openProtect()
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

func newProtectCamerasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cameras",
		Short: "Camera commands",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List Protect cameras",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openProtect()
			if err != nil {
				return err
			}
			cams, err := api.Cameras(cmd.Context())
			if err != nil {
				return mapAPIErr(err)
			}
			total := len(cams)
			start := rootOpts.offset
			if start < 0 {
				start = 0
			}
			if start > total {
				start = total
			}
			end := total
			if rootOpts.limit > 0 && start+rootOpts.limit < end {
				end = start + rootOpts.limit
			}
			page := cams[start:end]
			out := map[string]any{"totalCount": total, "count": len(page), "cameras": page}
			rows := make([][]string, 0, len(page))
			for _, c := range page {
				rows = append(rows, []string{c.Name, c.ModelKey, c.State, c.ID})
			}
			return printList(cmd, out, []string{"NAME", "MODEL", "STATE", "ID"}, rows, start, len(page), total)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <camera-id-or-name>",
		Short: "Get camera details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openProtect()
			if err != nil {
				return err
			}
			id, err := resolveProtectCamera(cmd.Context(), api, args[0])
			if err != nil {
				return err
			}
			cam, err := api.Camera(cmd.Context(), id)
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, cam, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", cam.ID, cam.Name, cam.State)
			})
		},
	})
	cmd.AddCommand(newProtectSnapshotCmd())
	cmd.AddCommand(newProtectStreamCmd())
	cmd.AddCommand(newProtectCameraRestartCmd())
	return cmd
}
