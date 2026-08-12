package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/config"
	"github.com/SPDG/unicli/internal/credstore"
	"github.com/SPDG/unicli/internal/exitcode"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage named UniFi gateway profiles",
	}
	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileUseCmd())
	cmd.AddCommand(newProfileShowCmd())
	cmd.AddCommand(newProfileSetCmd())
	cmd.AddCommand(newProfileDeleteCmd())
	return cmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured gateway profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			type row struct {
				Name     string `json:"name"`
				Host     string `json:"host"`
				Insecure bool   `json:"insecure"`
				Site     string `json:"site,omitempty"`
				Current  bool   `json:"current"`
			}
			rows := make([]row, 0, len(cfg.Profiles))
			for name, p := range cfg.Profiles {
				rows = append(rows, row{
					Name: name, Host: p.Host, Insecure: p.Insecure, Site: p.Site, Current: name == cfg.Current,
				})
			}
			return printValue(cmd, map[string]any{"current": cfg.Current, "profiles": rows}, func() {
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "(no profiles)")
					return
				}
				for _, r := range rows {
					mark := " "
					if r.Current {
						mark = "*"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s %s\t%s\tinsecure=%v\n", mark, r.Name, r.Host, r.Insecure)
				}
			})
		},
	}
}

func newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the current gateway profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			name := args[0]
			if _, ok := cfg.Profiles[name]; !ok {
				return exitf(exitcode.NotFound, "profile %q not found", name)
			}
			cfg.Current = name
			if err := config.Save(path, cfg); err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			return printValue(cmd, map[string]any{"current": name}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "current profile: %s\n", name)
			})
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show a profile (default: current)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			name := cfg.Current
			if len(args) == 1 {
				name = args[0]
			}
			if name == "" {
				return exitf(exitcode.Config, "no current profile")
			}
			p, ok := cfg.Profiles[name]
			if !ok {
				return exitf(exitcode.NotFound, "profile %q not found", name)
			}
			out := map[string]any{
				"name":     name,
				"host":     p.Host,
				"insecure": p.Insecure,
				"site":     p.Site,
				"current":  name == cfg.Current,
			}
			return printValue(cmd, out, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s host=%s insecure=%v site=%s\n", name, p.Host, p.Insecure, p.Site)
			})
		},
	}
}

func newProfileSetCmd() *cobra.Command {
	var insecure bool
	var site string
	cmd := &cobra.Command{
		Use:   "set <name> <host>",
		Short: "Create or update a gateway profile (does not set the API key)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			name, host := args[0], config.NormalizeHost(args[1])
			prev := cfg.Profiles[name]
			p := config.Profile{Host: host, Insecure: prev.Insecure, Site: prev.Site}
			if cmd.Flags().Changed("insecure") {
				p.Insecure = insecure
			}
			if cmd.Flags().Changed("site") {
				p.Site = site
			}
			cfg.Profiles[name] = p
			if cfg.Current == "" {
				cfg.Current = name
			}
			if err := config.Save(path, cfg); err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			return printValue(cmd, map[string]any{"name": name, "host": p.Host, "insecure": p.Insecure, "site": p.Site}, nil)
		},
	}
	cmd.Flags().BoolVar(&insecure, "insecure", false, "skip TLS verification")
	cmd.Flags().StringVar(&site, "site", "", "default Network site id")
	return cmd
}

func newProfileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a gateway profile and its stored API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig()
			if err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			name := args[0]
			if _, ok := cfg.Profiles[name]; !ok {
				return exitf(exitcode.NotFound, "profile %q not found", name)
			}
			delete(cfg.Profiles, name)
			if cfg.Current == name {
				cfg.Current = ""
				for n := range cfg.Profiles {
					cfg.Current = n
					break
				}
			}
			if err := config.Save(path, cfg); err != nil {
				return exitf(exitcode.Config, "%v", err)
			}
			if storePath, err := credPath(); err == nil {
				_ = credstore.New(storePath).Delete(name)
			}
			return printValue(cmd, map[string]any{"deleted": name, "current": cfg.Current}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", name)
			})
		},
	}
}
