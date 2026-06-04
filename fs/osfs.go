package fs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// defaultMaxReadBytes caps a single read so a huge file cannot be dumped into
// the model context.
const defaultMaxReadBytes int64 = 10 * 1024 * 1024

// Config configures an OSFileSystem.
type Config struct {
	Root         string // directory tools may touch (empty → current working dir)
	MaxReadBytes int64  // per-read byte cap (0 → default)
}

// OSFileSystem is the default FileSystem: real disk access confined to an
// absolute, symlink-resolved workspace root. Every untrusted path is contained
// to the root before any access, and symlink escapes are rejected.
type OSFileSystem struct {
	root     string // absolute, symlinks resolved
	maxBytes int64
}

// NewOSFileSystem builds an OSFileSystem. The root must exist; it is
// symlink-resolved once here and all later checks compare against the result.
func NewOSFileSystem(cfg Config) (*OSFileSystem, error) {
	root := cfg.Root
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("fs root %q: %w", root, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("fs root %q: %w", abs, err)
	}
	maxBytes := cfg.MaxReadBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxReadBytes
	}
	return &OSFileSystem{root: resolved, maxBytes: maxBytes}, nil
}

// Root returns the absolute workspace root.
func (w *OSFileSystem) Root() string { return w.root }

// ReadFile implements FileSystem.
func (w *OSFileSystem) ReadFile(_ context.Context, path string) ([]byte, error) {
	resolved, err := w.resolve(path)
	if err != nil {
		return nil, err
	}
	if _, err := w.statRegular(resolved); err != nil {
		return nil, err
	}
	return os.ReadFile(resolved)
}

// WriteFile implements FileSystem.
func (w *OSFileSystem) WriteFile(_ context.Context, path string, data []byte, opts WriteOptions) error {
	final, err := w.containWrite(path)
	if err != nil {
		return err
	}

	mode := opts.Mode
	if mode == "" {
		mode = ModeCreate
	}
	if _, statErr := os.Stat(final); statErr == nil && mode == ModeCreate {
		return fmt.Errorf("%s: %w", path, ErrExist)
	}

	if opts.CreateDirs {
		if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
			return fmt.Errorf("create parent dirs: %w", err)
		}
	}

	if mode == ModeAppend {
		f, err := os.OpenFile(final, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("open for append: %w", err)
		}
		defer func() { _ = f.Close() }()
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("append: %w", err)
		}
		return nil
	}
	return os.WriteFile(final, data, 0o644)
}

// List implements FileSystem.
func (w *OSFileSystem) List(_ context.Context, path string) ([]fs.DirEntry, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	resolved, err := w.resolve(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", path)
	}
	return os.ReadDir(resolved)
}

// Stat implements FileSystem.
func (w *OSFileSystem) Stat(_ context.Context, path string) (fs.FileInfo, error) {
	resolved, err := w.resolve(path)
	if err != nil {
		return nil, err
	}
	return os.Stat(resolved)
}

// Glob implements FileSystem. Patterns support ** (recursive) via doublestar
// and are evaluated against an fs.FS rooted at the workspace, so matches cannot
// escape containment. Results are workspace-relative, slash-separated paths.
func (w *OSFileSystem) Glob(_ context.Context, pattern string) ([]string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, errors.New("pattern is required")
	}
	if filepath.IsAbs(pattern) || hasDotDot(pattern) {
		return nil, fmt.Errorf("pattern must be workspace-relative without %q: %s", "..", pattern)
	}
	matches, err := doublestar.Glob(os.DirFS(w.root), filepath.ToSlash(pattern), doublestar.WithFilesOnly())
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", pattern, err)
	}
	return matches, nil
}

// resolve validates an untrusted path and returns the absolute,
// symlink-resolved path inside the workspace, or an error if it escapes
// containment or does not exist (EvalSymlinks requires existence).
func (w *OSFileSystem) resolve(p string) (string, error) {
	joined, err := w.contain(p)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", err
	}
	if !w.within(resolved) {
		return "", fmt.Errorf("path escapes workspace via symlink: %s", p)
	}
	return resolved, nil
}

// contain performs the lexical containment checks shared by read and write
// paths: reject null bytes, "..", and anything that joins outside the root.
func (w *OSFileSystem) contain(p string) (string, error) {
	if p == "" {
		return "", errors.New("path is required")
	}
	if strings.ContainsRune(p, '\x00') {
		return "", errors.New("path not allowed: contains null byte")
	}
	if hasDotDot(p) {
		return "", fmt.Errorf("path not allowed: %q contains %q", p, "..")
	}
	cleaned := filepath.Clean(p)
	joined := cleaned
	if !filepath.IsAbs(cleaned) {
		joined = filepath.Join(w.root, cleaned)
	}
	joined = filepath.Clean(joined)
	if !w.within(joined) {
		return "", fmt.Errorf("path escapes workspace: %s", p)
	}
	return joined, nil
}

// containWrite validates an untrusted path for a write, allowing the leaf to
// not yet exist. It resolves symlinks on the longest existing ancestor to catch
// a symlinked directory pointing outside the workspace, and refuses to write
// through a symlink leaf or over a directory.
func (w *OSFileSystem) containWrite(p string) (string, error) {
	joined, err := w.contain(p)
	if err != nil {
		return "", err
	}

	anc := joined
	for {
		if _, err := os.Lstat(anc); err == nil {
			break
		}
		parent := filepath.Dir(anc)
		if parent == anc {
			break
		}
		anc = parent
	}
	realAnc, err := filepath.EvalSymlinks(anc)
	if err != nil {
		return "", err
	}
	if !w.within(realAnc) {
		return "", fmt.Errorf("path escapes workspace via symlink: %s", p)
	}
	rel, err := filepath.Rel(anc, joined)
	if err != nil {
		return "", err
	}
	final := filepath.Join(realAnc, rel)
	if !w.within(final) {
		return "", fmt.Errorf("path escapes workspace: %s", p)
	}

	if fi, err := os.Lstat(final); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing to write through symlink: %s", p)
		}
		if fi.IsDir() {
			return "", fmt.Errorf("path is a directory: %s", p)
		}
	}
	return final, nil
}

// within reports whether absolute path p is inside the workspace root.
func (w *OSFileSystem) within(p string) bool {
	rel, err := filepath.Rel(w.root, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// statRegular enforces the size cap and rejects non-regular files.
func (w *OSFileSystem) statRegular(resolved string) (os.FileInfo, error) {
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("path is a directory, not a file")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("path is not a regular file")
	}
	if info.Size() > w.maxBytes {
		return nil, fmt.Errorf("file too large: %d bytes (limit: %d bytes)", info.Size(), w.maxBytes)
	}
	return info, nil
}

// hasDotDot reports whether any path segment is "..".
func hasDotDot(p string) bool {
	return slices.Contains(strings.Split(filepath.ToSlash(p), "/"), "..")
}
