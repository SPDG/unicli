package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags when releasing.
var Version = "0.0.0-dev"

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "unicli",
		Short:         "UniFi Network, Protect, and Access CLI",
		Long:          "Agent-friendly CLI for Ubiquiti UniFi Network, Protect, and Access.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newVersionCmd())
	return root
}

func Execute() error {
	root := NewRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print unicli version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), Version)
		},
	}
}
