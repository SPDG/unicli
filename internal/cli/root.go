package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/access"
	"github.com/SPDG/unicli/internal/client"
	"github.com/SPDG/unicli/internal/config"
	"github.com/SPDG/unicli/internal/credstore"
	"github.com/SPDG/unicli/internal/exitcode"
	"github.com/SPDG/unicli/internal/network"
	"github.com/SPDG/unicli/internal/output"
	"github.com/SPDG/unicli/internal/protect"
	"github.com/SPDG/unicli/internal/selectfields"
)

// Version is set at build time via -ldflags when releasing.
var Version = "0.0.0-dev"

type rootOptions struct {
	configPath      string
	profile         string
	host            string
	site            string
	insecure        bool
	insecureSet     bool
	jsonOut         bool
	plainOut        bool
	limit           int
	offset          int
	selectFields    string
	allowMutations  bool
	yes             bool
}

var rootOpts rootOptions

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "unicli",
		Short:         "UniFi Network, Protect, and Access CLI",
		Long:          "Agent-friendly CLI for Ubiquiti UniFi Network, Protect, and Access.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true

	root.PersistentFlags().StringVar(&rootOpts.configPath, "config", "", "config file (default: ~/.config/unicli/config.yaml)")
	root.PersistentFlags().StringVar(&rootOpts.profile, "profile", "", "named gateway profile")
	root.PersistentFlags().StringVar(&rootOpts.host, "host", "", "console URL or host (overrides profile/env partially)")
	root.PersistentFlags().StringVar(&rootOpts.site, "site", "", "Network site id (UUID)")
	root.PersistentFlags().BoolVar(&rootOpts.insecure, "insecure", false, "skip TLS certificate verification")
	root.PersistentFlags().BoolVar(&rootOpts.jsonOut, "json", false, "force JSON output")
	root.PersistentFlags().BoolVar(&rootOpts.plainOut, "plain", false, "force plain text output")
	root.PersistentFlags().IntVar(&rootOpts.limit, "limit", 25, "page size for list commands")
	root.PersistentFlags().IntVar(&rootOpts.offset, "offset", 0, "page offset for list commands")
	root.PersistentFlags().StringVar(&rootOpts.selectFields, "select", "", "comma-separated JSON fields to project")
	root.PersistentFlags().BoolVar(&rootOpts.allowMutations, "allow-mutations", false, "permit state-changing commands")
	root.PersistentFlags().BoolVar(&rootOpts.yes, "yes", false, "skip confirmation prompts for destructive mutations")

	_ = root.RegisterFlagCompletionFunc("profile", completeProfileNames)

	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		rootOpts.insecureSet = cmd.Flags().Changed("insecure")
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newAuthCmd())
	root.AddCommand(newProfileCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newSchemaCmd())
	root.AddCommand(newNetworkCmd())
	root.AddCommand(newProtectCmd())
	root.AddCommand(newAccessCmd())
	root.AddCommand(newCompletionCmd())
	return root
}

func Execute() error {
	root := NewRoot()
	if err := root.Execute(); err != nil {
		var xe *ExitError
		if errors.As(err, &xe) {
			if xe.Message != "" {
				fmt.Fprintln(os.Stderr, xe.Message)
			}
			os.Exit(xe.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("exit %d", e.Code)
}

func exitf(code int, format string, args ...any) error {
	return &ExitError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func configPath() (string, error) {
	if rootOpts.configPath != "" {
		return rootOpts.configPath, nil
	}
	return config.DefaultPath()
}

func loadConfig() (*config.File, string, error) {
	path, err := configPath()
	if err != nil {
		return nil, "", err
	}
	f, err := config.Load(path)
	return f, path, err
}

func credPath() (string, error) {
	return credstore.DefaultPath()
}

func resolveConnection() (*config.Resolved, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return nil, exitf(exitcode.Config, "%v", err)
	}
	storePath, err := credPath()
	if err != nil {
		return nil, exitf(exitcode.Config, "%v", err)
	}
	store := credstore.New(storePath)

	opts := config.ResolveOptions{
		Profile: rootOpts.profile,
		Host:    rootOpts.host,
		Site:    rootOpts.site,
		APIKey:  "", // env only via Resolve
	}
	if rootOpts.insecureSet {
		v := rootOpts.insecure
		opts.Insecure = &v
	}

	res, err := config.Resolve(cfg, opts, store.Get)
	if err != nil {
		if config.IsConfigError(err) {
			return nil, exitf(exitcode.Config, "%v", err)
		}
		return nil, exitf(exitcode.Config, "%v", err)
	}
	return res, nil
}

func newHTTPClient(res *config.Resolved) (*client.Client, error) {
	return client.New(res.Host, res.APIKey, res.Insecure)
}

func format() output.Format {
	return output.ResolveFormat(rootOpts.jsonOut, rootOpts.plainOut)
}

func printValue(cmd *cobra.Command, v any, plain func()) error {
	projected, err := selectfields.Apply(v, rootOpts.selectFields)
	if err != nil {
		return exitf(exitcode.Usage, "select: %v", err)
	}
	if output.WantJSON(format(), os.Stdout) || rootOpts.selectFields != "" {
		// Field projection is JSON-oriented; always emit JSON when --select is set.
		return output.WriteJSON(cmd.OutOrStdout(), projected)
	}
	if plain != nil {
		plain()
		return nil
	}
	return output.WriteJSON(cmd.OutOrStdout(), projected)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print unicli version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if output.WantJSON(format(), os.Stdout) {
				return output.WriteJSON(cmd.OutOrStdout(), map[string]string{"version": Version})
			}
			fmt.Fprintln(cmd.OutOrStdout(), Version)
			return nil
		},
	}
}

func newSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Print machine-readable command schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema := map[string]any{
				"name":    "unicli",
				"version": Version,
				"exit_codes": map[string]int{
					"ok":               exitcode.OK,
					"usage":            exitcode.Usage,
					"empty":            exitcode.Empty,
					"auth_required":    exitcode.AuthRequired,
					"not_found":        exitcode.NotFound,
					"permission":       exitcode.Permission,
					"rate_limited":     exitcode.RateLimited,
					"retryable":        exitcode.Retryable,
					"config":           exitcode.Config,
					"unsupported":      exitcode.Unsupported,
					"mutation_blocked": exitcode.MutationBlocked,
					"input_required":   exitcode.InputRequired,
					"cancelled":        exitcode.Cancelled,
				},
				"flags": map[string]any{
					"allow_mutations": "required for state-changing commands",
					"yes":             "skip confirmation for destructive mutations when non-interactive",
					"select":          "comma-separated field projection for JSON output",
					"limit":           "list page size",
					"offset":          "list page offset",
				},
				"commands": []map[string]any{
					{"name": "version"},
					{"name": "schema"},
					{"name": "doctor"},
					{"name": "auth login"},
					{"name": "auth status"},
					{"name": "auth logout"},
					{"name": "profile list"},
					{"name": "profile use"},
					{"name": "profile show"},
					{"name": "profile set"},
					{"name": "profile delete"},
					{"name": "network info"},
					{"name": "network sites list"},
					{"name": "network devices list"},
					{"name": "network devices get"},
					{"name": "network devices stats"},
					{"name": "network devices restart", "mutation": true, "confirmation_required": true},
					{"name": "network ports cycle", "mutation": true, "confirmation_required": true},
					{"name": "network clients list"},
					{"name": "network clients get"},
					{"name": "network clients authorize", "mutation": true, "confirmation_required": true},
					{"name": "network clients unauthorize", "mutation": true, "confirmation_required": true},
					{"name": "protect info"},
					{"name": "protect cameras list"},
					{"name": "protect cameras get"},
					{"name": "access info"},
					{"name": "access doors list"},
					{"name": "access doors get"},
					{"name": "access doors unlock", "mutation": true, "confirmation_required": true},
					{"name": "access users list"},
					{"name": "access users get"},
					{"name": "completion install"},
					{"name": "unicli-mcp (stdio MCP wrapper)"},
				},
			}
			return output.WriteJSON(cmd.OutOrStdout(), schema)
		},
	}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check connectivity and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := resolveConnection()
			report := map[string]any{}
			if err != nil {
				report["ok"] = false
				report["error"] = err.Error()
				_ = output.WriteJSON(cmd.OutOrStdout(), report)
				return err
			}
			report["ok"] = true
			report["host"] = res.Host
			report["profile"] = res.Profile
			report["source"] = res.Source
			report["insecure"] = res.Insecure
			report["site"] = res.Site
			report["api_key_set"] = res.APIKey != ""

			c, err := newHTTPClient(res)
			if err != nil {
				report["ok"] = false
				report["error"] = err.Error()
				_ = output.WriteJSON(cmd.OutOrStdout(), report)
				return exitf(exitcode.Config, "%v", err)
			}
			api := network.New(c)
			info, err := api.Info(cmd.Context())
			if err != nil {
				report["ok"] = false
				report["error"] = err.Error()
				_ = output.WriteJSON(cmd.OutOrStdout(), report)
				return mapAPIErr(err)
			}
			report["network_version"] = info.ApplicationVersion

			if pinfo, perr := protect.New(c).Info(cmd.Context()); perr == nil {
				report["protect_version"] = pinfo.ApplicationVersion
			} else {
				report["protect_error"] = perr.Error()
			}

			if _, aerr := access.New(c).Info(cmd.Context()); aerr == nil {
				report["access_available"] = true
			} else {
				report["access_available"] = false
				report["access_error"] = aerr.Error()
			}

			return printValue(cmd, report, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "ok host=%s profile=%s network=%s\n", res.Host, res.Profile, info.ApplicationVersion)
			})
		},
	}
}

func mapAPIErr(err error) error {
	var unavailable client.AppUnavailableError
	if errors.As(err, &unavailable) {
		return exitf(exitcode.Unsupported, "UniFi application not available on this console: %v", err)
	}
	var ae client.APIError
	if errors.As(err, &ae) {
		switch {
		case ae.Status == 401 || ae.Status == 403:
			return exitf(exitcode.AuthRequired, "%v", err)
		case ae.Status == 404:
			return exitf(exitcode.NotFound, "%v", err)
		case ae.Status == 429:
			return exitf(exitcode.RateLimited, "%v", err)
		case ae.Status >= 500:
			return exitf(exitcode.Retryable, "%v", err)
		default:
			return exitf(exitcode.Usage, "%v", err)
		}
	}
	return exitf(exitcode.Retryable, "%v", err)
}

func resolveSiteID(ctx context.Context, api *network.API, preferred string) (string, error) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		return preferred, nil
	}
	page, err := api.Sites(ctx, 0, 25)
	if err != nil {
		return "", err
	}
	if len(page.Data) == 0 {
		return "", exitf(exitcode.Empty, "no sites found")
	}
	return page.Data[0].ID, nil
}

// readAPIKeyFromStdin reads a single line / whole stdin without echo prompts.
func readAPIKeyFromStdin(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", exitf(exitcode.InputRequired, "API key required on stdin (printf %%s \"$UNIFI_API_KEY\" | unicli auth login)")
	}
	return key, nil
}
