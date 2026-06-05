package fs

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/sho0pi/agenttools/tool"
)

const defaultMaxGrepResults = 200

// GrepArgs are the fs_grep arguments.
type GrepArgs struct {
	Pattern       string `json:"pattern"`
	Root          string `json:"root"`
	Glob          string `json:"glob"`
	Regex         bool   `json:"regex"`
	CaseSensitive bool   `json:"case_sensitive"`
	ContextLines  int    `json:"context_lines"`
	MaxResults    int    `json:"max_results"`
}

// NewGrepTool returns the fs_grep tool backed by fsys, or an error if fsys is nil.
func NewGrepTool(fsys FileSystem) (tool.Tool, error) {
	if fsys == nil {
		return nil, fmt.Errorf("fs: grep: FileSystem must not be nil")
	}
	return tool.NewTypedTool(
		"fs_grep",
		"Search file contents for a pattern and return matching lines as 'path:line: text'. "+
			"The primary tool for finding symbols, errors, TODOs, and usage sites. Restrict with "+
			"glob (e.g. '**/*.go') to search fewer files, and set max_results to stay within budget. "+
			"To find files by name use fs_glob; to read a specific file use fs_read.",
		tool.Object(map[string]*tool.Property{
			"pattern":        {Type: "string", Description: "Text or regex to search for."},
			"root":           {Type: "string", Description: "Directory to search under (default workspace root)."},
			"glob":           {Type: "string", Description: "Only search files matching this glob (e.g. '**/*.go')."},
			"regex":          {Type: "boolean", Description: "Treat pattern as a regular expression (RE2 syntax)."},
			"case_sensitive": {Type: "boolean", Description: "Case-sensitive match (default false)."},
			"context_lines":  {Type: "integer", Description: "Lines of context before/after each match."},
			"max_results":    {Type: "integer", Description: fmt.Sprintf("Cap on matches returned (default %d).", defaultMaxGrepResults)},
		}, "pattern"),
		func(ctx context.Context, args GrepArgs) (tool.Result, error) {
			return grep(ctx, fsys, args)
		},
	), nil
}

func grep(ctx context.Context, fsys FileSystem, args GrepArgs) (tool.Result, error) {
	pattern := strings.TrimSpace(args.Pattern)
	if pattern == "" {
		return tool.Result{}, fmt.Errorf("pattern is required")
	}

	matcher, err := buildMatcher(args)
	if err != nil {
		return tool.Result{}, err
	}

	globPat := strings.TrimSpace(args.Glob)
	if globPat == "" {
		globPat = "**/*"
	}
	if root := strings.TrimSpace(args.Root); root != "" {
		globPat = path.Join(root, globPat)
	}
	files, err := fsys.Glob(ctx, globPat)
	if err != nil {
		return tool.Result{}, err
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxGrepResults
	}
	ctxLines := args.ContextLines
	if ctxLines < 0 {
		ctxLines = 0
	}

	var sb strings.Builder
	total := 0
	shown := 0
	capped := false

	for _, f := range files {
		data, err := fsys.ReadFile(ctx, f)
		if err != nil || !utf8.Valid(data) {
			continue // skip unreadable/binary files
		}
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		for i, line := range lines {
			if !matcher(line) {
				continue
			}
			total++
			if shown >= maxResults {
				capped = true
				continue
			}
			writeMatch(&sb, f, lines, i, ctxLines)
			shown++
		}
	}

	if total == 0 {
		return tool.Result{Content: fmt.Sprintf("No matches for %q.", pattern)}, nil
	}
	if capped {
		fmt.Fprintf(&sb, "[showing %d of %d matches]", shown, total)
	}
	return tool.Result{
		Content: strings.TrimRight(sb.String(), "\n"),
		Data:    map[string]any{"pattern": pattern, "matches": total, "truncated": capped},
	}, nil
}

// buildMatcher returns a per-line predicate for the search.
func buildMatcher(args GrepArgs) (func(string) bool, error) {
	if args.Regex {
		expr := args.Pattern
		if !args.CaseSensitive {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		return re.MatchString, nil
	}
	if args.CaseSensitive {
		needle := args.Pattern
		return func(s string) bool { return strings.Contains(s, needle) }, nil
	}
	needle := strings.ToLower(args.Pattern)
	return func(s string) bool { return strings.Contains(strings.ToLower(s), needle) }, nil
}

// writeMatch appends the matched line (path:line: text) plus optional context
// lines (path-line- text) to sb.
func writeMatch(sb *strings.Builder, file string, lines []string, idx, ctxLines int) {
	start := max(0, idx-ctxLines)
	end := min(len(lines)-1, idx+ctxLines)
	for j := start; j <= end; j++ {
		sep := "-"
		if j == idx {
			sep = ":"
		}
		fmt.Fprintf(sb, "%s%s%d%s %s\n", file, ":", j+1, sep, lines[j])
	}
}
