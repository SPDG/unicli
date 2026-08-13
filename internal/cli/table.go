package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/SPDG/unicli/internal/output"
)

func printList(cmd *cobra.Command, v any, headers []string, rows [][]string, offset, shown, total int) error {
	return printValue(cmd, v, func() {
		_ = output.WriteTable(cmd.OutOrStdout(), headers, rows)
		fmt.Fprintln(cmd.OutOrStdout(), output.PageFooter(offset, shown, total))
	})
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
