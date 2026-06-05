package fs

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/sho0pi/agenttools/tool"
)

const defaultMaxListEntries = 500

// ListArgs are the fs_list arguments.
type ListArgs struct {
	Path          string `json:"path"`
	Recursive     bool   `json:"recursive"`
	IncludeHidden bool   `json:"include_hidden"`
	MaxDepth      int    `json:"max_depth"`
	MaxEntries    int    `json:"max_entries"`
	Sort          string `json:"sort"`
}

// NewListTool returns the fs_list tool backed by fsys, or an error if fsys is nil.
func NewListTool(fsys FileSystem) (tool.Tool, error) {
	if fsys == nil {
		return nil, fmt.Errorf("fs: list: FileSystem must not be nil")
	}
	return tool.NewTypedTool(
		"fs_list",
		"List directory contents in the workspace. Use to discover what files exist "+
			"before reading them. Set max_depth/max_entries to avoid huge listings — do not "+
			"recursively list a large tree when fs_glob (match by pattern) or fs_grep (search "+
			"contents) would find the target directly.",
		tool.Object(map[string]*tool.Property{
			"path":           {Type: "string", Description: "Directory to list, relative to the workspace root (default: root)."},
			"recursive":      {Type: "boolean", Description: "Descend into subdirectories."},
			"include_hidden": {Type: "boolean", Description: "Include entries starting with a dot."},
			"max_depth":      {Type: "integer", Description: "Maximum recursion depth (default 1; >1 implies recursive)."},
			"max_entries":    {Type: "integer", Description: fmt.Sprintf("Cap on entries returned (default %d).", defaultMaxListEntries)},
			"sort":           {Type: "string", Description: "Sort order.", Enum: []string{"name", "modified", "size"}},
		}),
		func(ctx context.Context, args ListArgs) (tool.Result, error) {
			return listDir(ctx, fsys, args)
		},
	), nil
}

type listEntry struct {
	rel   string
	isDir bool
	size  int64
	mod   int64
}

func listDir(ctx context.Context, fsys FileSystem, args ListArgs) (tool.Result, error) {
	base := strings.TrimSpace(args.Path)
	if base == "" {
		base = "."
	}
	maxDepth := args.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	}
	if args.Recursive && args.MaxDepth <= 0 {
		maxDepth = 1 << 30 // effectively unlimited; capped by max_entries
	}
	maxEntries := args.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultMaxListEntries
	}

	var entries []listEntry
	truncated := false

	var walk func(dir, rel string, depth int) error
	walk = func(dir, rel string, depth int) error {
		des, err := fsys.List(ctx, dir)
		if err != nil {
			return err
		}
		for _, de := range des {
			name := de.Name()
			if !args.IncludeHidden && strings.HasPrefix(name, ".") {
				continue
			}
			childRel := path.Join(rel, name)
			e := listEntry{rel: childRel, isDir: de.IsDir()}
			if fi, err := de.Info(); err == nil {
				e.size = fi.Size()
				e.mod = fi.ModTime().UnixNano()
			}
			if len(entries) >= maxEntries {
				truncated = true
				return nil
			}
			entries = append(entries, e)
			if de.IsDir() && depth < maxDepth {
				if err := walk(path.Join(dir, name), childRel, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(base, "", 1); err != nil {
		return tool.Result{}, err
	}

	sortEntries(entries, args.Sort)

	var sb strings.Builder
	for _, e := range entries {
		if e.isDir {
			fmt.Fprintf(&sb, "[DIR]  %s/\n", e.rel)
		} else {
			fmt.Fprintf(&sb, "       %s (%d bytes)\n", e.rel, e.size)
		}
	}
	if truncated {
		fmt.Fprintf(&sb, "[showing first %d entries; more exist]", maxEntries)
	} else {
		fmt.Fprintf(&sb, "[%d entries]", len(entries))
	}

	return tool.Result{
		Content: sb.String(),
		Data:    map[string]any{"path": base, "count": len(entries), "truncated": truncated},
	}, nil
}

// sortEntries orders entries: directories first, then by the requested key
// (name by default). Within each group the chosen key applies.
func sortEntries(entries []listEntry, key string) {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.isDir != b.isDir {
			return a.isDir
		}
		switch key {
		case "modified":
			return a.mod > b.mod
		case "size":
			return a.size > b.size
		default:
			return a.rel < b.rel
		}
	})
}
