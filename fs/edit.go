package fs

import (
	"context"
	"fmt"
	"strings"

	"github.com/sho0pi/agenttools/tool"
)

// EditArgs are the fs_edit arguments.
type EditArgs struct {
	Path                 string `json:"path"`
	OldString            string `json:"old_string"`
	NewString            string `json:"new_string"`
	ExpectedReplacements int    `json:"expected_replacements"`
	CreateBackup         bool   `json:"create_backup"`
	DryRun               bool   `json:"dry_run"`
}

// NewEditTool returns the fs_edit tool backed by fsys, or an error if fsys is nil.
func NewEditTool(fsys FileSystem) (tool.Tool, error) {
	if fsys == nil {
		return nil, fmt.Errorf("fs: edit: FileSystem must not be nil")
	}
	return tool.NewTypedTool(
		"fs_edit",
		"Make a precise edit to an existing file by replacing an exact string. Use for "+
			"localized changes to code/config — prefer this over fs_write overwrite for any "+
			"change that is not a full rewrite. old_string must match exactly and be unique "+
			"unless expected_replacements is set. Read the file first so the match is accurate; "+
			"use dry_run if unsure.",
		tool.Object(map[string]*tool.Property{
			"path":                  {Type: "string", Description: "File to edit, relative to the workspace root."},
			"old_string":            {Type: "string", Description: "Exact text to replace; include surrounding context to be unique."},
			"new_string":            {Type: "string", Description: "Replacement text."},
			"expected_replacements": {Type: "integer", Description: "Require exactly this many matches (default 1)."},
			"create_backup":         {Type: "boolean", Description: "Save a .bak copy before editing."},
			"dry_run":               {Type: "boolean", Description: "Report what would change without writing."},
		}, "path", "old_string", "new_string"),
		func(ctx context.Context, args EditArgs) (tool.Result, error) {
			return editFile(ctx, fsys, args)
		},
	), nil
}

func editFile(ctx context.Context, fsys FileSystem, args EditArgs) (tool.Result, error) {
	if args.OldString == "" {
		return tool.Result{}, fmt.Errorf("old_string is required")
	}
	if args.OldString == args.NewString {
		return tool.Result{}, fmt.Errorf("old_string and new_string are identical; nothing to change")
	}

	data, err := fsys.ReadFile(ctx, args.Path)
	if err != nil {
		return tool.Result{}, err
	}
	content := string(data)

	count := strings.Count(content, args.OldString)
	want := args.ExpectedReplacements
	if want <= 0 {
		want = 1
	}
	switch {
	case count == 0:
		return tool.Result{}, fmt.Errorf("old_string not found in %s", args.Path)
	case count != want:
		return tool.Result{}, fmt.Errorf("old_string matched %d time(s) in %s, expected %d; add context or set expected_replacements", count, args.Path, want)
	}

	updated := strings.ReplaceAll(content, args.OldString, args.NewString)

	if args.DryRun {
		return tool.Result{
			Content: fmt.Sprintf("[dry run] would make %d replacement(s) in %s.", count, args.Path),
			Data:    map[string]any{"path": args.Path, "replacements": count, "dry_run": true},
		}, nil
	}

	if args.CreateBackup {
		if err := fsys.WriteFile(ctx, args.Path+".bak", data, WriteOptions{Mode: ModeOverwrite}); err != nil {
			return tool.Result{}, fmt.Errorf("backup: %w", err)
		}
	}
	if err := fsys.WriteFile(ctx, args.Path, []byte(updated), WriteOptions{Mode: ModeOverwrite}); err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: fmt.Sprintf("Edited %s (%d replacement(s)).", args.Path, count),
		Data:    map[string]any{"path": args.Path, "replacements": count},
	}, nil
}
