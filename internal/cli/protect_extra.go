package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/protect"
)

func newProtectNVRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nvr",
		Short: "Show the Protect NVR",
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openProtect()
			if err != nil {
				return err
			}
			nvr, err := api.NVR(cmd.Context())
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, nvr, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", anyString(nvr["name"]), anyString(nvr["modelKey"]), anyString(nvr["id"]))
			})
		},
	}
}

func newProtectLiveviewsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "liveviews", Short: "Live view layouts"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List live views",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProtectDevices(cmd, "liveviews")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <liveview-id-or-name>",
		Short: "Get a live view",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openProtect()
			if err != nil {
				return err
			}
			items, err := api.Devices(cmd.Context(), "liveviews")
			if err != nil {
				return mapAPIErr(err)
			}
			id, err := resolveProtectDevice(args[0], items)
			if err != nil {
				return err
			}
			item, err := api.Device(cmd.Context(), "liveviews", id)
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, item, nil)
		},
	})
	return cmd
}

func newProtectDevicesCmd(collection, short string) *cobra.Command {
	cmd := &cobra.Command{Use: collection, Short: short}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List " + collection,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProtectDevices(cmd, collection)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get <id-or-name>",
		Short: "Get one " + strings.TrimSuffix(collection, "s"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			api, err := openProtect()
			if err != nil {
				return err
			}
			items, err := api.Devices(cmd.Context(), collection)
			if err != nil {
				return mapAPIErr(err)
			}
			id, err := resolveProtectDevice(args[0], items)
			if err != nil {
				return err
			}
			item, err := api.Device(cmd.Context(), collection, id)
			if err != nil {
				return mapAPIErr(err)
			}
			return printValue(cmd, item, nil)
		},
	})
	return cmd
}

func listProtectDevices(cmd *cobra.Command, collection string) error {
	api, err := openProtect()
	if err != nil {
		return err
	}
	items, err := api.Devices(cmd.Context(), collection)
	if err != nil {
		return mapAPIErr(err)
	}
	var filtered []map[string]any
	for _, item := range items {
		if matchName(protect.DeviceName(item), rootOpts.filterName) {
			filtered = append(filtered, item)
		}
	}
	total := len(filtered)
	start, end := sliceBounds(total)
	page := filtered[start:end]
	out := map[string]any{"totalCount": total, "count": len(page), collection: page}
	rows := make([][]string, 0, len(page))
	for _, item := range page {
		rows = append(rows, []string{
			protect.DeviceName(item),
			anyString(item["modelKey"]),
			anyString(item["state"]),
			protect.DeviceID(item),
		})
	}
	return printList(cmd, out, []string{"NAME", "MODEL", "STATE", "ID"}, rows, start, len(page), total)
}

func newProtectSnapshotCmd() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "snapshot <camera-id-or-name>",
		Short: "Download a JPEG snapshot",
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
			jpeg, err := api.Snapshot(cmd.Context(), id)
			if err != nil {
				return mapAPIErr(err)
			}
			if outPath == "" || outPath == "-" {
				if outPath == "" && (outputIsJSON() || term.IsTerminal(int(os.Stdout.Fd()))) {
					return exitf(exitcode.Usage, "pass --output PATH (JPEG is binary; use -o - to write to stdout)")
				}
				_, err = cmd.OutOrStdout().Write(jpeg)
				return err
			}
			if err := os.WriteFile(outPath, jpeg, 0o644); err != nil {
				return err
			}
			meta := map[string]any{"cameraId": id, "path": outPath, "bytes": len(jpeg), "contentType": "image/jpeg"}
			return printValue(cmd, meta, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes)\n", outPath, len(jpeg))
			})
		},
	}
	cmd.Flags().StringVarP(&outPath, "output", "o", "", "file path, or - for stdout")
	return cmd
}

func newProtectStreamCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stream <camera-id-or-name>",
		Short: "Show RTSPS URLs (tokens redacted unless --include-secrets)",
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
			streams, err := api.RTSPS(cmd.Context(), id)
			if err != nil {
				return mapAPIErr(err)
			}
			if !rootOpts.includeSecrets {
				streams = redactRTSPS(streams)
			}
			out := map[string]any{"cameraId": id, "rtsps": streams}
			return printValue(cmd, out, func() {
				for _, q := range []string{"high", "medium", "low", "package"} {
					if v := anyString(streams[q]); v != "" && v != "<nil>" {
						fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", q, v)
					}
				}
			})
		},
	}
}

func newProtectCameraRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <camera-id-or-name>",
		Short: "Restart a camera (mutation)",
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
			return runMutation(cmd, "protect cameras restart", fmt.Sprintf("Restart camera %s?", id), func() (any, error) {
				if err := api.RestartCamera(cmd.Context(), id); err != nil {
					return nil, err
				}
				return map[string]any{"status": "ok", "id": id, "action": "restart"}, nil
			})
		},
	}
}

func newProtectCameraUpdateCmd() *cobra.Command {
	var fromJSON string
	cmd := &cobra.Command{
		Use:   "update <camera-id-or-name>",
		Short: "PATCH camera settings from JSON (mutation)",
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
			body, err := readJSONBody(fromJSON)
			if err != nil {
				return err
			}
			return runMutation(cmd, "protect cameras update", fmt.Sprintf("Update camera %s?", id), func() (any, error) {
				return api.PatchCamera(cmd.Context(), id, body)
			})
		},
	}
	jsonFlag(cmd, &fromJSON)
	_ = cmd.MarkFlagRequired("from-json")
	return cmd
}

func newProtectCameraSetCmd() *cobra.Command {
	var hdr, videoMode, micEnabled string
	var micVolume int
	cmd := &cobra.Command{
		Use:   "set <camera-id-or-name>",
		Short: "Set HDR, video mode, or microphone (mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patch := map[string]any{}
			if hdr != "" {
				patch["hdrType"] = hdr
			}
			if videoMode != "" {
				patch["videoMode"] = videoMode
			}
			if cmd.Flags().Changed("mic-volume") {
				if micVolume < 0 || micVolume > 100 {
					return exitf(exitcode.Usage, "--mic-volume must be 0-100")
				}
				patch["micVolume"] = micVolume
			}
			if micEnabled != "" {
				switch strings.ToLower(micEnabled) {
				case "true", "on", "1", "yes":
					patch["isMicEnabled"] = true
				case "false", "off", "0", "no":
					patch["isMicEnabled"] = false
				default:
					return exitf(exitcode.Usage, "invalid --mic-enabled %q", micEnabled)
				}
			}
			if len(patch) == 0 {
				return exitf(exitcode.Usage, "set at least one of --hdr, --video-mode, --mic-volume, --mic-enabled")
			}
			api, err := openProtect()
			if err != nil {
				return err
			}
			id, err := resolveProtectCamera(cmd.Context(), api, args[0])
			if err != nil {
				return err
			}
			return runMutation(cmd, "protect cameras set", fmt.Sprintf("Set camera %s %v?", id, patch), func() (any, error) {
				return api.PatchCamera(cmd.Context(), id, patch)
			})
		},
	}
	cmd.Flags().StringVar(&hdr, "hdr", "", "HDR mode: auto, on, off")
	cmd.Flags().StringVar(&videoMode, "video-mode", "", "video mode: default, sport, slowShutter, highFps")
	cmd.Flags().IntVar(&micVolume, "mic-volume", -1, "microphone volume 0-100")
	cmd.Flags().StringVar(&micEnabled, "mic-enabled", "", "true or false")
	return cmd
}

func resolveProtectCamera(ctx context.Context, api *protect.API, arg string) (string, error) {
	cams, err := api.Cameras(ctx)
	if err != nil {
		return "", mapAPIErr(err)
	}
	return resolveID(arg, cams, func(c protect.Camera) string { return c.Name }, func(c protect.Camera) string { return c.ID })
}

func resolveProtectDevice(arg string, items []map[string]any) (string, error) {
	return resolveID(arg, items, protect.DeviceName, protect.DeviceID)
}

func sliceBounds(total int) (int, int) {
	start := rootOpts.offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := total
	if !rootOpts.allPages && rootOpts.limit > 0 && start+rootOpts.limit < end {
		end = start + rootOpts.limit
	}
	return start, end
}

func outputIsJSON() bool {
	return rootOpts.jsonOut || rootOpts.selectFields != ""
}

func redactRTSPS(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		s, ok := v.(string)
		if !ok || s == "" {
			out[k] = v
			continue
		}
		out[k] = redactStreamURL(s)
	}
	return out
}

func redactStreamURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "[redacted]"
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "rtsps"
	}
	return scheme + "://" + u.Host + "/[redacted]"
}
