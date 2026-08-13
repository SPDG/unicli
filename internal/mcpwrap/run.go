package mcpwrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/SPDG/unicli/internal/exitcode"
)

// mutationTokens are unicli subcommands that change console state.
var mutationTokens = map[string]struct{}{
	"restart":     {},
	"cycle":       {},
	"unlock":      {},
	"authorize":   {},
	"unauthorize": {},
	"create":      {},
	"update":      {},
	"delete":      {},
	"enable":      {},
	"disable":     {},
	"logging":     {},
	"generate":    {},
}

// Result is a unicli invocation outcome.
type Result struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// Runner executes the unicli binary. Tests can override CommandContext.
type Runner struct {
	Bin            string
	CommandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func NewRunner() *Runner {
	return &Runner{Bin: FindUnicli()}
}

func FindUnicli() string {
	if p := strings.TrimSpace(os.Getenv("UNICLI_BIN")); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "unicli"+exeExt())
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	if p, err := exec.LookPath("unicli"); err == nil {
		return p
	}
	return "unicli"
}

func exeExt() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}

func IsMutation(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if _, ok := mutationTokens[a]; ok {
			return true
		}
	}
	return false
}

func ensureJSON(args []string) []string {
	for _, a := range args {
		if a == "--json" {
			return args
		}
	}
	return append([]string{"--json"}, args...)
}

func ensureYes(args []string) []string {
	for _, a := range args {
		if a == "--yes" {
			return args
		}
	}
	return append(args, "--yes")
}

func ensureAllowMutations(args []string) []string {
	for _, a := range args {
		if a == "--allow-mutations" {
			return args
		}
	}
	return append(args, "--allow-mutations")
}

// Run executes unicli with args. Mutations require allowMutations.
func (r *Runner) Run(ctx context.Context, args []string, allowMutations bool) (*Result, error) {
	if r == nil {
		r = NewRunner()
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("args required (try: schema, doctor, network devices list)")
	}
	if IsMutation(args) && !allowMutations {
		return &Result{
			ExitCode: exitcode.MutationBlocked,
			Stderr:   "mutation blocked: pass allow_mutations=true (MCP) or --allow-mutations on the CLI",
		}, nil
	}
	args = ensureJSON(args)
	if IsMutation(args) {
		args = ensureAllowMutations(args)
		args = ensureYes(args)
	}

	bin := r.Bin
	if bin == "" {
		bin = FindUnicli()
	}
	cc := r.CommandContext
	if cc == nil {
		cc = exec.CommandContext
	}
	cmd := cc(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := &Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return res, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		res.ExitCode = ee.ExitCode()
		return res, nil
	}
	return res, err
}
