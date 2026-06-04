// Package shellexec provides the shell_exec tool and the Executor provider it
// delegates to. Running shell commands is the most dangerous agent capability,
// so the execution boundary — timeout, working-directory jail, environment
// policy, output caps, and (in stricter implementations) sandboxing and command
// allowlists — lives entirely in the Executor. The tool only translates model
// arguments into a CommandSpec.
package shellexec

import (
	"context"
	"time"
)

// CommandSpec describes one command to run. Command is a shell command line
// (the Executor runs it through a shell); the remaining fields scope its
// execution.
type CommandSpec struct {
	Command string            // shell command line, e.g. "go test ./..."
	Cwd     string            // working directory (Executor-relative); empty → Executor default
	Env     map[string]string // extra environment variables
	Stdin   string            // data piped to the command's stdin
	Timeout time.Duration     // hard timeout; 0 → Executor default
}

// CommandResult is the outcome of a CommandSpec.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

// Executor runs commands under a policy it owns. Implementations range from a
// local process runner (LocalExecutor) to container/VM-isolated runners. Run
// returns an error only when the command could not be started or the policy
// rejected it; a command that runs and exits non-zero is reported via
// CommandResult.ExitCode, not error.
type Executor interface {
	Run(ctx context.Context, spec CommandSpec) (CommandResult, error)
}
