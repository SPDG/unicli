package mcpwrap

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SPDG/unicli/internal/cli"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type runInput struct {
	Args            []string `json:"args" jsonschema:"unicli arguments after the binary, e.g. [\"network\",\"devices\",\"list\"]"`
	AllowMutations  bool     `json:"allow_mutations,omitempty" jsonschema:"required for restart, cycle, unlock, authorize"`
	Profile         string   `json:"profile,omitempty" jsonschema:"named gateway profile"`
}

type doctorInput struct {
	Profile string `json:"profile,omitempty" jsonschema:"named gateway profile"`
}

type schemaInput struct{}

func NewServer(runner *Runner) *mcp.Server {
	if runner == nil {
		runner = NewRunner()
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "unicli",
		Version: cli.Version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "unicli_schema",
		Description: "Discover unicli commands, flags, and exit codes. Call this before other unicli tools.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ schemaInput) (*mcp.CallToolResult, *Result, error) {
		return toolResult(runner.Run(ctx, []string{"schema"}, false))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "unicli_doctor",
		Description: "Check UniFi console connectivity and which apps are available.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in doctorInput) (*mcp.CallToolResult, *Result, error) {
		args := []string{"doctor"}
		if in.Profile != "" {
			args = append([]string{"--profile", in.Profile}, args...)
		}
		return toolResult(runner.Run(ctx, args, false))
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "unicli_run",
		Description: "Run a unicli command. Always JSON. Discover commands via unicli_schema. Mutations need allow_mutations=true. Never pass API keys in args.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, *Result, error) {
		args := append([]string(nil), in.Args...)
		if in.Profile != "" {
			args = append([]string{"--profile", in.Profile}, args...)
		}
		return toolResult(runner.Run(ctx, args, in.AllowMutations))
	})

	return server
}

func toolResult(res *Result, err error) (*mcp.CallToolResult, *Result, error) {
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, nil, nil
	}
	raw, _ := json.MarshalIndent(res, "", "  ")
	out := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
	}
	if res.ExitCode != 0 {
		out.IsError = true
	}
	return out, res, nil
}

func RunStdio(ctx context.Context) error {
	server := NewServer(nil)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("unicli-mcp: %w", err)
	}
	return nil
}
