package mcpwrap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SPDG/unicli/internal/exitcode"
)

func fakeUnicli(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "unicli")
	script := "#!/bin/sh\nprintf '{\"ok\":true,\"args\":\"%s\"}\\n' \"$*\"\nexit 0\n"
	if runtime.GOOS == "windows" {
		path += ".bat"
		script = "@echo {\"ok\":true}\r\n"
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIsMutation(t *testing.T) {
	if !IsMutation([]string{"network", "devices", "restart", "abc"}) {
		t.Fatal("restart")
	}
	if IsMutation([]string{"network", "devices", "list"}) {
		t.Fatal("list is not a mutation")
	}
	if !IsMutation([]string{"console", "reboot"}) {
		t.Fatal("reboot")
	}
	if !IsMutation([]string{"network", "wifi", "create"}) {
		t.Fatal("create")
	}
	if IsMutation([]string{"network", "wifi", "list"}) {
		t.Fatal("list is not a mutation")
	}
}

func TestRunInjectsJSON(t *testing.T) {
	bin := fakeUnicli(t)
	r := &Runner{Bin: bin}
	res, err := r.Run(context.Background(), []string{"network", "info"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("%+v", res)
	}
	if !strings.Contains(res.Stdout, "--json") {
		t.Fatalf("expected --json in args, got %s", res.Stdout)
	}
}

func TestRunBlocksMutations(t *testing.T) {
	r := &Runner{Bin: fakeUnicli(t)}
	res, err := r.Run(context.Background(), []string{"network", "devices", "restart", "x"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != exitcode.MutationBlocked {
		t.Fatalf("code=%d", res.ExitCode)
	}
}

func TestRunAllowsMutations(t *testing.T) {
	r := &Runner{Bin: fakeUnicli(t)}
	res, err := r.Run(context.Background(), []string{"network", "devices", "restart", "x"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("%+v", res)
	}
	if !strings.Contains(res.Stdout, "--allow-mutations") || !strings.Contains(res.Stdout, "--yes") {
		t.Fatalf("expected mutation flags, got %s", res.Stdout)
	}
}

func TestRunCapturesExitCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unicli")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho fail-out\necho fail-err >&2\nexit 11\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Runner{Bin: path}
	res, err := r.Run(context.Background(), []string{"access", "info"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 11 {
		t.Fatalf("code=%d stderr=%s", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "fail-out") || !strings.Contains(res.Stderr, "fail-err") {
		t.Fatalf("%+v", res)
	}
}

func TestFindUnicliEnv(t *testing.T) {
	t.Setenv("UNICLI_BIN", "/tmp/custom-unicli")
	if got := FindUnicli(); got != "/tmp/custom-unicli" {
		t.Fatalf("got %s", got)
	}
}

func TestCommandContextOverride(t *testing.T) {
	r := &Runner{
		Bin: "unicli",
		CommandContext: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, fakeUnicli(t), args...)
		},
	}
	res, err := r.Run(context.Background(), []string{"schema"}, false)
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("%v %+v", err, res)
	}
}
