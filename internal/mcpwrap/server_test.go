package mcpwrap

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServerUnicliRun(t *testing.T) {
	runner := &Runner{Bin: fakeUnicli(t)}
	server := NewServer(runner)
	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()

	srvSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srvSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	cliSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cliSession.Close() })

	res, err := cliSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "unicli_run",
		Arguments: map[string]any{"args": []string{"network", "info"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res)
	}
}
