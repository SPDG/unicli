package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/SPDG/unicli/internal/exitcode"
	"golang.org/x/term"
)

func requireMutations(action string) error {
	if !rootOpts.allowMutations {
		return exitf(exitcode.MutationBlocked,
			"mutation blocked: %s requires --allow-mutations", action)
	}
	return nil
}

func requireConfirm(prompt string) error {
	if rootOpts.yes {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return exitf(exitcode.InputRequired,
			"confirmation required for %q; re-run with --yes", prompt)
	}
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return exitf(exitcode.InputRequired, "confirmation aborted: %v", err)
	}
	ans := strings.TrimSpace(strings.ToLower(line))
	if ans != "y" && ans != "yes" {
		return exitf(exitcode.Cancelled, "aborted")
	}
	return nil
}
