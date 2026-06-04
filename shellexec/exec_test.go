package shellexec

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sho0pi/agenttools/tool"
)

func newExec(t *testing.T, cfg Config) *LocalExecutor {
	t.Helper()
	if cfg.WorkRoot == "" {
		cfg.WorkRoot = t.TempDir()
	}
	e, err := NewLocalExecutor(cfg)
	if err != nil {
		t.Fatalf("NewLocalExecutor: %v", err)
	}
	return e
}

func TestNew_NilExecutor(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil Executor")
	}
}

func TestLocalExecutor_Run(t *testing.T) {
	e := newExec(t, Config{})
	ctx := context.Background()

	t.Run("stdout and exit 0", func(t *testing.T) {
		res, err := e.Run(ctx, CommandSpec{Command: "echo hello"})
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode != 0 || strings.TrimSpace(res.Stdout) != "hello" {
			t.Fatalf("got %+v", res)
		}
	})

	t.Run("non-zero exit is not an error", func(t *testing.T) {
		res, err := e.Run(ctx, CommandSpec{Command: "exit 3"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ExitCode != 3 {
			t.Fatalf("exit code = %d, want 3", res.ExitCode)
		}
	})

	t.Run("stdin", func(t *testing.T) {
		res, err := e.Run(ctx, CommandSpec{Command: "cat", Stdin: "piped"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(res.Stdout) != "piped" {
			t.Fatalf("stdout = %q", res.Stdout)
		}
	})

	t.Run("env passthrough", func(t *testing.T) {
		res, err := e.Run(ctx, CommandSpec{Command: "echo $FOO", Env: map[string]string{"FOO": "bar"}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(res.Stdout) != "bar" {
			t.Fatalf("stdout = %q, want bar", res.Stdout)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		// A POSIX busy-loop blocks without relying on a `sleep` binary (the test
		// environment shims sleep), so the context timeout is what stops it.
		res, err := e.Run(ctx, CommandSpec{Command: "while :; do :; done", Timeout: 150 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		if !res.TimedOut {
			t.Fatalf("expected TimedOut, got %+v", res)
		}
	})
}

func TestLocalExecutor_CwdJail(t *testing.T) {
	e := newExec(t, Config{})
	if _, err := e.Run(context.Background(), CommandSpec{Command: "pwd", Cwd: "../../"}); err == nil {
		t.Fatal("cwd escaping the work root should error")
	}
}

func TestLocalExecutor_OutputCap(t *testing.T) {
	e := newExec(t, Config{MaxOutputBytes: 100})
	res, err := e.Run(context.Background(), CommandSpec{Command: "head -c 1000 /dev/zero | tr '\\0' a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stdout) > 100 {
		t.Fatalf("stdout not capped: %d bytes", len(res.Stdout))
	}
}

func TestLocalExecutor_EnvNotInheritedByDefault(t *testing.T) {
	t.Setenv("SECRET_TOKEN", "do-not-leak")
	e := newExec(t, Config{})
	res, err := e.Run(context.Background(), CommandSpec{Command: "echo \"[$SECRET_TOKEN]\""})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Stdout, "do-not-leak") {
		t.Fatalf("host env leaked into command: %q", res.Stdout)
	}
}

func TestShellExecTool(t *testing.T) {
	tr, err := New(newExec(t, Config{}))
	if err != nil {
		t.Fatal(err)
	}
	exec := func(args Args) (tool.Result, error) {
		raw, _ := json.Marshal(args)
		return tr.Execute(context.Background(), raw)
	}

	res, err := exec(Args{Command: "echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "hi") || res.Data["exit_code"] != 0 {
		t.Fatalf("got content=%q data=%v", res.Content, res.Data)
	}

	if _, err := exec(Args{Command: "echo x", Background: true}); err == nil {
		t.Fatal("background=true should error (defer to shell_session)")
	}
	if _, err := exec(Args{Command: "  "}); err == nil {
		t.Fatal("blank command should error")
	}
}
