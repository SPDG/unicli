package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/exitcode"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate or install shell completion scripts",
	}

	bash := &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
		},
	}
	zsh := &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		},
	}
	fish := &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		},
	}

	install := &cobra.Command{
		Use:   "install [bash|zsh|fish]",
		Short: "Install completion script for the current user",
		Args:  cobra.MaximumNArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := "bash"
			if len(args) == 1 {
				shell = args[0]
			} else if s := strings.TrimSpace(os.Getenv("SHELL")); s != "" {
				shell = filepath.Base(s)
			}
			path, err := installCompletion(cmd.Root(), shell)
			if err != nil {
				return err
			}
			return printValue(cmd, map[string]any{"status": "ok", "shell": shell, "path": path}, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "installed %s completion to %s\n", shell, path)
				switch shell {
				case "bash":
					fmt.Fprintln(cmd.OutOrStdout(), "restart your shell (or: source ~/.bashrc)")
				case "zsh":
					fmt.Fprintln(cmd.OutOrStdout(), "restart your shell (ensure compinit is enabled)")
				}
			})
		},
	}

	cmd.AddCommand(bash, zsh, fish, install)
	return cmd
}

func installCompletion(root *cobra.Command, shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", exitf(exitcode.Config, "%v", err)
	}
	var (
		dir, file string
		write     func(*os.File) error
	)
	switch shell {
	case "bash":
		dir = filepath.Join(home, ".local/share/bash-completion/completions")
		file = filepath.Join(dir, "unicli")
		write = func(f *os.File) error { return root.GenBashCompletionV2(f, true) }
	case "zsh":
		dir = filepath.Join(home, ".zsh/completions")
		file = filepath.Join(dir, "_unicli")
		write = func(f *os.File) error { return root.GenZshCompletion(f) }
	case "fish":
		dir = filepath.Join(home, ".config/fish/completions")
		file = filepath.Join(dir, "unicli.fish")
		write = func(f *os.File) error { return root.GenFishCompletion(f, true) }
	default:
		return "", exitf(exitcode.Usage, "unsupported shell %q (bash|zsh|fish)", shell)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", exitf(exitcode.Config, "%v", err)
	}
	f, err := os.Create(file)
	if err != nil {
		return "", exitf(exitcode.Config, "%v", err)
	}
	defer f.Close()
	if err := write(f); err != nil {
		return "", err
	}
	return file, nil
}
