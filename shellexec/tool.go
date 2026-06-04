package shellexec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sho0pi/agenttools/tool"
)

// Args are the shell_exec arguments.
type Args struct {
	Command    string            `json:"command"`
	Cwd        string            `json:"cwd"`
	TimeoutSec int               `json:"timeout_sec"`
	Env        map[string]string `json:"env"`
	Stdin      string            `json:"stdin"`
	Background bool              `json:"background"`
}

// New returns the shell_exec tool backed by exec, or an error if exec is nil.
func New(exec Executor) (tool.Tool, error) {
	if exec == nil {
		return nil, fmt.Errorf("shellexec: Executor must not be nil")
	}
	return tool.NewTypedTool(
		"shell_exec",
		"Run a shell command in the sandboxed workspace and return its stdout, stderr, "+
			"and exit code. Use for build/test/lint/package tasks that need native tools. "+
			"POWERFUL AND RISKY: prefer a dedicated tool when one exists (fs_* for files, "+
			"git_* for git, http_request for APIs) instead of shelling out. Never run "+
			"destructive commands (rm -rf, dd, mkfs). For long-running processes use "+
			"shell_session, not this tool.",
		tool.Object(map[string]*tool.Property{
			"command":     {Type: "string", Description: "Shell command line to run."},
			"cwd":         {Type: "string", Description: "Working directory, relative to the workspace root."},
			"timeout_sec": {Type: "integer", Description: "Timeout in seconds (Executor default if unset)."},
			"env":         {Type: "object", Description: "Extra environment variables for the command."},
			"stdin":       {Type: "string", Description: "Data to pipe to the command's stdin."},
			"background":  {Type: "boolean", Description: "Unsupported here; use shell_session for background processes."},
		}, "command"),
		func(ctx context.Context, args Args) (tool.Result, error) {
			return run(ctx, exec, args)
		},
	), nil
}

func run(ctx context.Context, exec Executor, args Args) (tool.Result, error) {
	if strings.TrimSpace(args.Command) == "" {
		return tool.Result{}, fmt.Errorf("command is required")
	}
	if args.Background {
		return tool.Result{}, fmt.Errorf("background execution is not supported by shell_exec; use the shell_session tool")
	}

	res, err := exec.Run(ctx, CommandSpec{
		Command: args.Command,
		Cwd:     args.Cwd,
		Env:     args.Env,
		Stdin:   args.Stdin,
		Timeout: time.Duration(args.TimeoutSec) * time.Second,
	})
	if err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: render(res),
		Data: map[string]any{
			"exit_code": res.ExitCode, "timed_out": res.TimedOut,
			"stdout": res.Stdout, "stderr": res.Stderr,
		},
	}, nil
}

// render formats a CommandResult into compact, model-readable text.
func render(res CommandResult) string {
	var b strings.Builder
	if res.TimedOut {
		b.WriteString("[timed out]\n")
	}
	fmt.Fprintf(&b, "exit code: %d\n", res.ExitCode)
	if out := strings.TrimRight(res.Stdout, "\n"); out != "" {
		fmt.Fprintf(&b, "\nstdout:\n%s\n", out)
	}
	if errOut := strings.TrimRight(res.Stderr, "\n"); errOut != "" {
		fmt.Fprintf(&b, "\nstderr:\n%s\n", errOut)
	}
	return strings.TrimRight(b.String(), "\n")
}
