package fs

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/sho0pi/agenttools/tool"
)

// ReadArgs are the fs_read arguments.
type ReadArgs struct {
	Path               string `json:"path"`
	StartLine          int    `json:"start_line"`
	EndLine            int    `json:"end_line"`
	MaxBytes           int    `json:"max_bytes"`
	IncludeLineNumbers bool   `json:"include_line_numbers"`
}

// NewReadTool returns the fs_read tool backed by fsys, or an error if fsys is nil.
func NewReadTool(fsys FileSystem) (tool.Tool, error) {
	if fsys == nil {
		return nil, fmt.Errorf("fs: read: FileSystem must not be nil")
	}
	return tool.NewTypedTool(
		"fs_read",
		"Read a text file from the workspace. Use to inspect code, configs, or logs "+
			"before acting. For large files, request a line range (start_line/end_line) "+
			"or set max_bytes instead of reading the whole file. Paths are workspace-relative; "+
			"to search inside files use fs_grep, to find files use fs_glob.",
		tool.Object(map[string]*tool.Property{
			"path":                 {Type: "string", Description: "File path, relative to the workspace root."},
			"start_line":           {Type: "integer", Description: "1-based first line to return (default 1)."},
			"end_line":             {Type: "integer", Description: "1-based last line to return (default: end of file)."},
			"max_bytes":            {Type: "integer", Description: "Cap on bytes read before slicing lines."},
			"include_line_numbers": {Type: "boolean", Description: "Prefix each returned line with its number."},
		}, "path"),
		func(ctx context.Context, args ReadArgs) (tool.Result, error) {
			return readFile(ctx, fsys, args)
		},
	), nil
}

func readFile(ctx context.Context, fsys FileSystem, args ReadArgs) (tool.Result, error) {
	data, err := fsys.ReadFile(ctx, args.Path)
	if err != nil {
		return tool.Result{}, err
	}
	if args.MaxBytes > 0 && len(data) > args.MaxBytes {
		data = data[:args.MaxBytes]
	}
	if !utf8.Valid(data) {
		return tool.Result{}, fmt.Errorf("file is not valid UTF-8 (binary?): %s", args.Path)
	}

	text := strings.TrimSuffix(string(data), "\n")
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	total := len(lines)

	start := args.StartLine
	if start <= 0 {
		start = 1
	}
	if start > total {
		return tool.Result{
			Content: fmt.Sprintf("[no lines in range; file has %d lines]", total),
			Data:    map[string]any{"path": args.Path, "total_lines": total},
		}, nil
	}
	end := total
	if args.EndLine > 0 && args.EndLine < end {
		end = args.EndLine
	}

	var sb strings.Builder
	for i := start - 1; i < end; i++ {
		if args.IncludeLineNumbers {
			fmt.Fprintf(&sb, "%d: %s\n", i+1, lines[i])
		} else {
			sb.WriteString(lines[i])
			sb.WriteByte('\n')
		}
	}

	return tool.Result{
		Content: strings.TrimSuffix(sb.String(), "\n"),
		Data: map[string]any{
			"path": args.Path, "total_lines": total,
			"start_line": start, "end_line": end,
			"truncated": end < total || start > 1,
		},
	}, nil
}
