package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/config"
	"github.com/SPDG/unicli/internal/credstore"
	"github.com/SPDG/unicli/internal/exitcode"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage API keys for gateway profiles",
	}
	cmd.AddCommand(newAuthLoginCmd())
	cmd.AddCommand(newAuthStatusCmd())
	cmd.AddCommand(newAuthLogoutCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var (
		profile  string
		host     string
		insecure bool
	)
	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Store an API key for a profile (key on stdin)",
		Long:    "Read the API key from stdin and bind it to a named profile. Never pass the key on argv.",
		Example: `  printf %s "$UNIFI_API_KEY" | unicli auth login --profile home --host https://192.168.1.1 --insecure`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			profile = strings.TrimSpace(profile)
			if profile == "" {
				profile = strings.TrimSpace(rootOpts.profile)
			}
			if profile == "" {
				profile = strings.TrimSpace(cfg.Current)
			}
			if profile == "" {
				profile = "default"
			}

			key, err := readAPIKeyFromStdin(cmd.InOrStdin())
			if err != nil {
				return err
			}

			host = strings.TrimSpace(host)
			if host == "" {
				host = strings.TrimSpace(rootOpts.host)
			}
			if host == "" {
				host = strings.TrimSpace(os.Getenv(config.EnvHost))
			}
			if existing, ok := cfg.Profiles[profile]; ok && host == "" {
				host = existing.Host
			}
			if host == "" {
				return exitf(exitcode.Config, "host required (--host or %s) when creating profile %q", config.EnvHost, profile)
			}

			ins := insecure
			if !cmd.Flags().Changed("insecure") {
				if existing, ok := cfg.Profiles[profile]; ok {
					ins = existing.Insecure
				} else if v, ok := config.ParseInsecureEnv(os.Getenv(config.EnvInsecure)); ok {
					ins = v
				}
			}

			prevSite := ""
			if existing, ok := cfg.Profiles[profile]; ok {
				prevSite = existing.Site
			}
			cfg.Profiles[profile] = config.Profile{
				Host:     config.NormalizeHost(host),
				Insecure: ins,
				Site:     prevSite,
			}
			if cfg.Current == "" {
				cfg.Current = profile
			}
			if err := config.Save(path, cfg); err != nil {
				return exitf(exitcode.Config, "save config: %v", err)
			}

			storePath, err := credPath()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			if err := credstore.New(storePath).Set(profile, key); err != nil {
				return exitf(exitcode.Config, "store key: %v", err)
			}

			return printValue(cmd, map[string]any{
				"status":  "ok",
				"profile": profile,
				"host":    cfg.Profiles[profile].Host,
			}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "stored credentials for profile %s (%s)\n", profile, cfg.Profiles[profile].Host)
			})
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name (default: current or \"default\")")
	cmd.Flags().StringVar(&host, "host", "", "console URL or host")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "skip TLS verification for this profile")
	_ = cmd.RegisterFlagCompletionFunc("profile", completeProfileNames)
	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether credentials are configured (never prints the key)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			profile := strings.TrimSpace(rootOpts.profile)
			if profile == "" {
				profile = strings.TrimSpace(os.Getenv(config.EnvProfile))
			}
			if profile == "" {
				profile = cfg.Current
			}

			storePath, err := credPath()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			_, keyErr := credstore.New(storePath).Get(profile)
			envKey := strings.TrimSpace(os.Getenv(config.EnvAPIKey)) != ""

			out := map[string]any{
				"profile":         profile,
				"profile_key_set": keyErr == nil,
				"env_api_key_set": envKey,
				"env_host_set":    strings.TrimSpace(os.Getenv(config.EnvHost)) != "",
				"current":         cfg.Current,
			}
			return printValue(cmd, out, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "profile=%s profile_key=%v env_key=%v\n", profile, keyErr == nil, envKey)
			})
		},
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored API key for the selected profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			profile := strings.TrimSpace(rootOpts.profile)
			if profile == "" {
				profile = cfg.Current
			}
			if profile == "" {
				return exitf(exitcode.Config, "no profile selected")
			}
			storePath, err := credPath()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			if err := credstore.New(storePath).Delete(profile); err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			return printValue(cmd, map[string]any{"status": "ok", "profile": profile}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "removed stored key for %s\n", profile)
			})
		},
	}
}
