package shellexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultTimeout   = 30 * time.Second
	defaultMaxOutput = 1 << 20 // 1 MiB per stream
	defaultShell     = "/bin/sh"
)

// Config configures a LocalExecutor.
type Config struct {
	// WorkRoot bounds working directories. A CommandSpec.Cwd is resolved
	// relative to it and may not escape it. Empty → current working directory.
	WorkRoot string
	// Shell is the shell used to run command lines (default /bin/sh).
	Shell string
	// DefaultTimeout applies when a CommandSpec sets none (default 30s).
	DefaultTimeout time.Duration
	// MaxOutputBytes caps each of stdout/stderr (default 1 MiB).
	MaxOutputBytes int
	// InheritEnv passes the parent process environment to commands. When false
	// (default), only CommandSpec.Env is provided — safer, no host secrets leak.
	InheritEnv bool
}

// LocalExecutor runs commands as local child processes via a shell. It is the
// default Executor for trusted environments: it enforces a timeout, a
// working-directory jail, an output cap, and an environment policy, but does
// NOT sandbox the process. Use a container/VM-backed Executor for untrusted
// agents.
type LocalExecutor struct {
	root       string
	shell      string
	timeout    time.Duration
	maxOutput  int
	inheritEnv bool
}

// NewLocalExecutor builds a LocalExecutor, or an error if WorkRoot is set but
// invalid.
func NewLocalExecutor(cfg Config) (*LocalExecutor, error) {
	root := cfg.WorkRoot
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("work root %q: %w", root, err)
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("work root %q is not a directory", abs)
	}
	e := &LocalExecutor{
		root:       abs,
		shell:      cfg.Shell,
		timeout:    cfg.DefaultTimeout,
		maxOutput:  cfg.MaxOutputBytes,
		inheritEnv: cfg.InheritEnv,
	}
	if e.shell == "" {
		e.shell = defaultShell
	}
	if e.timeout <= 0 {
		e.timeout = defaultTimeout
	}
	if e.maxOutput <= 0 {
		e.maxOutput = defaultMaxOutput
	}
	return e, nil
}

// Run implements Executor.
func (e *LocalExecutor) Run(ctx context.Context, spec CommandSpec) (CommandResult, error) {
	if strings.TrimSpace(spec.Command) == "" {
		return CommandResult{}, errors.New("command is required")
	}

	dir, err := e.resolveCwd(spec.Cwd)
	if err != nil {
		return CommandResult{}, err
	}

	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = e.timeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, e.shell, "-c", spec.Command)
	cmd.Dir = dir
	cmd.Env = e.buildEnv(spec.Env)
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &cappedWriter{buf: &stdout, max: e.maxOutput}
	cmd.Stderr = &cappedWriter{buf: &stderr, max: e.maxOutput}

	runErr := cmd.Run()
	res := CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}

	if runCtx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitCode = -1
		return res, nil
	}
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			res.ExitCode = ee.ExitCode()
			return res, nil // ran, non-zero exit
		}
		return CommandResult{}, fmt.Errorf("start command: %w", runErr)
	}
	return res, nil
}

// resolveCwd validates that cwd stays within the work root.
func (e *LocalExecutor) resolveCwd(cwd string) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return e.root, nil
	}
	joined := cwd
	if !filepath.IsAbs(joined) {
		joined = filepath.Join(e.root, cwd)
	}
	joined = filepath.Clean(joined)
	rel, err := filepath.Rel(e.root, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("cwd escapes work root: %s", cwd)
	}
	if fi, err := os.Stat(joined); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("cwd is not a directory: %s", cwd)
	}
	return joined, nil
}

// buildEnv assembles the command environment per policy. When not inheriting,
// it provides only a minimal safe base (PATH, HOME) plus the caller's extra
// vars, so commands can still find binaries while host secrets are not leaked.
// The returned slice is always non-nil: a nil cmd.Env would make os/exec
// inherit the full parent environment, defeating the policy.
func (e *LocalExecutor) buildEnv(extra map[string]string) []string {
	var env []string
	if e.inheritEnv {
		env = os.Environ()
	} else {
		env = []string{"PATH=" + pathEnv()}
		if home := os.Getenv("HOME"); home != "" {
			env = append(env, "HOME="+home)
		}
	}
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// pathEnv returns the parent PATH, or a sane default if unset. PATH is not a
// secret, and commands need it to locate binaries.
func pathEnv() string {
	if p := os.Getenv("PATH"); p != "" {
		return p
	}
	return "/usr/local/bin:/usr/bin:/bin"
}

// cappedWriter writes to buf until max bytes, then silently drops the rest so a
// runaway command cannot exhaust memory.
type cappedWriter struct {
	buf *bytes.Buffer
	max int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if remaining := w.max - w.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			w.buf.Write(p[:remaining])
		} else {
			w.buf.Write(p)
		}
	}
	return len(p), nil // report full write so the process isn't blocked
}
