// Package search provides the web_search tool: a DuckDuckGo search via the
// ddg-search CLI (github.com/Djarvur/ddg-search). It returns result metadata
// only (titles, URLs, snippets) — use the web_fetch tool to read a page.
package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/sho0pi/agenttools/tool"
)

const (
	defaultResults = 5
	maxResults     = 10
)

// Args are the search arguments.
type Args struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

// New returns the web_search tool wired to provider, or an error if provider is nil.
func New(provider Provider) (tool.Tool, error) {
	if provider == nil {
		return nil, fmt.Errorf("search: provider must not be nil")
	}
	return tool.NewTypedTool(
		"web_search",
		"Search the web using DuckDuckGo. No API key required. "+
			"Returns titles, URLs, and snippets for the top results. "+
			"To read a result's full page, pass its URL to web_fetch.",
		schema(),
		func(ctx context.Context, args Args) (tool.Result, error) {
			return run(ctx, provider, args)
		},
	), nil
}

// Provider executes the search and returns raw output. Injected for testing.
type Provider func(ctx context.Context, query string, max int) (string, error)

func schema() *tool.Schema {
	return tool.Object(map[string]*tool.Property{
		"query": {
			Type:        "string",
			Description: "Search query, e.g. 'best Go web frameworks 2025'",
		},
		"max_results": {
			Type:        "number",
			Description: fmt.Sprintf("Max results to return (default: %d, max: %d)", defaultResults, maxResults),
		},
	}, "query")
}

func run(ctx context.Context, provider Provider, args Args) (tool.Result, error) {
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return tool.Result{}, fmt.Errorf("query is required")
	}

	n := defaultResults
	if args.MaxResults > 0 {
		n = min(args.MaxResults, maxResults)
	}

	out, err := provider(ctx, query, n)
	if err != nil {
		return tool.Result{}, fmt.Errorf("ddg-search: %w", err)
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return tool.Result{Content: fmt.Sprintf("No results found for %q.", query)}, nil
	}
	return tool.Result{
		Content: out,
		Data:    map[string]any{"query": query, "max_results": n},
	}, nil
}
