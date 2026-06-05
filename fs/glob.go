package fs

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/sho0pi/agenttools/tool"
)

const defaultMaxGlobResults = 500

// GlobArgs are the fs_glob arguments.
type GlobArgs struct {
	Pattern    string `json:"pattern"`
	Root       string `json:"root"`
	MaxResults int    `json:"max_results"`
}

// NewGlobTool returns the fs_glob tool backed by fsys, or an error if fsys is nil.
func NewGlobTool(fsys FileSystem) (tool.Tool, error) {
	if fsys == nil {
		return nil, fmt.Errorf("fs: glob: FileSystem must not be nil")
	}
	return tool.NewTypedTool(
		"fs_glob",
		"Find files by name pattern using glob syntax. Use to discover files before "+
			"reading them — faster than fs_grep for name-based discovery. Supports ** for "+
			"recursive matching (e.g. '**/*.go', 'src/**/*.json'). Restrict with root to "+
			"search a sub-tree. Do NOT use when you need to search file contents: use "+
			"fs_grep instead.",
		tool.Object(map[string]*tool.Property{
			"pattern":     {Type: "string", Description: "Glob pattern, workspace-relative (e.g. '**/*.go')."},
			"root":        {Type: "string", Description: "Sub-directory to search under (default: workspace root)."},
			"max_results": {Type: "integer", Description: fmt.Sprintf("Cap on returned paths (default %d).", defaultMaxGlobResults)},
		}, "pattern"),
		func(ctx context.Context, args GlobArgs) (tool.Result, error) {
			return globFiles(ctx, fsys, args)
		},
	), nil
}

func globFiles(ctx context.Context, fsys FileSystem, args GlobArgs) (tool.Result, error) {
	pattern := strings.TrimSpace(args.Pattern)
	if pattern == "" {
		return tool.Result{}, fmt.Errorf("pattern is required")
	}

	if root := strings.TrimSpace(args.Root); root != "" {
		pattern = path.Join(root, pattern)
	}

	matches, err := fsys.Glob(ctx, pattern)
	if err != nil {
		return tool.Result{}, err
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxGlobResults
	}

	capped := false
	if len(matches) > maxResults {
		matches = matches[:maxResults]
		capped = true
	}

	if len(matches) == 0 {
		return tool.Result{
			Content: fmt.Sprintf("No files match %q.", args.Pattern),
			Data:    map[string]any{"pattern": args.Pattern, "count": 0},
		}, nil
	}

	content := strings.Join(matches, "\n")
	if capped {
		content += fmt.Sprintf("\n[showing %d of more results]", maxResults)
	}

	return tool.Result{
		Content: content,
		Data:    map[string]any{"pattern": args.Pattern, "count": len(matches), "truncated": capped},
	}, nil
}
