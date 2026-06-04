// Package fs provides the filesystem tools (fs_read, fs_write, fs_edit,
// fs_list, fs_grep) and the FileSystem provider they delegate to.
//
// Every path argument is untrusted — it originates from the model, which
// ingests attacker-controlled web_search/web_fetch results and message text.
// The security boundary (path jail, symlink containment, size caps) lives in
// the FileSystem implementation, not in the tools, so it is enforced in one
// auditable place. The default implementation, OSFileSystem, confines all
// access to a configured workspace root.
package fs

import (
	"context"
	"errors"
	"io/fs"
)

// WriteMode controls how WriteFile treats an existing file.
type WriteMode string

const (
	// ModeCreate writes only when the file does not already exist.
	ModeCreate WriteMode = "create"
	// ModeOverwrite replaces the file's entire contents.
	ModeOverwrite WriteMode = "overwrite"
	// ModeAppend appends to the file, creating it if absent.
	ModeAppend WriteMode = "append"
)

// WriteOptions configures a WriteFile call.
type WriteOptions struct {
	Mode       WriteMode // defaults to ModeCreate when empty
	CreateDirs bool      // create parent directories as needed
}

// ErrExist is returned by WriteFile in ModeCreate when the target already
// exists, so tools can report it distinctly.
var ErrExist = errors.New("file already exists")

// FileSystem is the backend every fs_* tool delegates to. Implementations own
// the security boundary: path containment to a workspace, symlink safety, and
// read/list size caps. All paths are workspace-relative (absolute paths are
// accepted only if they resolve inside the workspace).
//
// The interface is intentionally small; richer tool behaviour (line slicing,
// exact-string edits, content search) is built on top of these primitives in
// the tool layer.
type FileSystem interface {
	// ReadFile returns the file's bytes, enforcing the implementation's size cap.
	ReadFile(ctx context.Context, path string) ([]byte, error)
	// WriteFile creates, overwrites, or appends per opts.
	WriteFile(ctx context.Context, path string, data []byte, opts WriteOptions) error
	// List returns the directory entries at path (non-recursive).
	List(ctx context.Context, path string) ([]fs.DirEntry, error)
	// Stat returns metadata for a single path.
	Stat(ctx context.Context, path string) (fs.FileInfo, error)
	// Glob returns workspace-relative paths matching pattern. Implementations
	// support ** for recursive matching.
	Glob(ctx context.Context, pattern string) ([]string, error)
}
