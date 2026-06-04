package fs

import (
	"context"
	"errors"
	"fmt"

	"github.com/sho0pi/agenttools/tool"
)

// WriteArgs are the fs_write arguments.
type WriteArgs struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	Mode       string `json:"mode"`
	CreateDirs bool   `json:"create_dirs"`
	Backup     bool   `json:"backup"`
}

// NewWriteTool returns the fs_write tool backed by fsys, or an error if fsys is nil.
func NewWriteTool(fsys FileSystem) (tool.Tool, error) {
	if fsys == nil {
		return nil, fmt.Errorf("fs: write: FileSystem must not be nil")
	}
	return tool.NewTypedTool(
		"fs_write",
		"Write content to a file in the workspace. Use to create new files or save "+
			"generated output. Mutating: 'overwrite' replaces the whole file — prefer fs_edit "+
			"for small changes to an existing file, and prefer 'append' over 'overwrite' when "+
			"adding to logs/notes. Do not overwrite a file you have not read.",
		tool.Object(map[string]*tool.Property{
			"path":        {Type: "string", Description: "File path, relative to the workspace root."},
			"content":     {Type: "string", Description: "Content to write."},
			"mode":        {Type: "string", Description: "create (default; fails if exists), overwrite, or append.", Enum: []string{"create", "overwrite", "append"}},
			"create_dirs": {Type: "boolean", Description: "Create parent directories as needed."},
			"backup":      {Type: "boolean", Description: "Save a .bak copy before overwriting an existing file."},
		}, "path", "content"),
		func(ctx context.Context, args WriteArgs) (tool.Result, error) {
			return writeFile(ctx, fsys, args)
		},
	), nil
}

func writeFile(ctx context.Context, fsys FileSystem, args WriteArgs) (tool.Result, error) {
	mode := WriteMode(args.Mode)
	switch mode {
	case "", ModeCreate:
		mode = ModeCreate
	case ModeOverwrite, ModeAppend:
	default:
		return tool.Result{}, fmt.Errorf("invalid mode %q (want create, overwrite, or append)", args.Mode)
	}

	existed := false
	if _, err := fsys.Stat(ctx, args.Path); err == nil {
		existed = true
	}

	// Back up before an overwrite of an existing file.
	if args.Backup && existed && mode == ModeOverwrite {
		old, err := fsys.ReadFile(ctx, args.Path)
		if err != nil {
			return tool.Result{}, fmt.Errorf("backup: read existing: %w", err)
		}
		if err := fsys.WriteFile(ctx, args.Path+".bak", old, WriteOptions{Mode: ModeOverwrite, CreateDirs: args.CreateDirs}); err != nil {
			return tool.Result{}, fmt.Errorf("backup: write .bak: %w", err)
		}
	}

	err := fsys.WriteFile(ctx, args.Path, []byte(args.Content), WriteOptions{Mode: mode, CreateDirs: args.CreateDirs})
	if errors.Is(err, ErrExist) {
		return tool.Result{}, fmt.Errorf("%s already exists; use mode 'overwrite' or 'append' to change it", args.Path)
	}
	if err != nil {
		return tool.Result{}, err
	}

	verb := map[WriteMode]string{ModeCreate: "Created", ModeOverwrite: "Wrote", ModeAppend: "Appended to"}[mode]
	return tool.Result{
		Content: fmt.Sprintf("%s %s (%d bytes).", verb, args.Path, len(args.Content)),
		Data:    map[string]any{"path": args.Path, "bytes": len(args.Content), "mode": string(mode), "created": !existed},
	}, nil
}
