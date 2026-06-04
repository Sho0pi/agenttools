package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sho0pi/agenttools/tool"
)

// Action names a memory operation.
type Action string

const (
	ActionStore  Action = "store"
	ActionSearch Action = "search"
	ActionUpdate Action = "update"
	ActionForget Action = "forget"
	ActionPurge  Action = "purge"
	ActionExport Action = "export"
)

var allActions = []string{
	string(ActionStore), string(ActionSearch), string(ActionUpdate),
	string(ActionForget), string(ActionPurge), string(ActionExport),
}

// Args are the memory tool arguments. Required fields depend on Action.
type Args struct {
	Action   string         `json:"action"`
	Key      string         `json:"key"`
	Query    string         `json:"query"`
	Content  string         `json:"content"`
	Category string         `json:"category"`
	Limit    int            `json:"limit"`
	TTL      string         `json:"ttl"`
	Metadata map[string]any `json:"metadata"`
}

// New returns the memory tool backed by store, or an error if store is nil.
func New(store Store) (tool.Tool, error) {
	if store == nil {
		return nil, fmt.Errorf("memory: Store must not be nil")
	}
	return tool.NewTypedTool(
		"memory",
		"Save and recall durable facts across sessions (user preferences, project "+
			"decisions, stable context). action is one of: store, search, update, forget, "+
			"purge, export. Store only information that will matter later — do NOT use it as "+
			"a scratchpad for the current task, and do not save secrets or info already in "+
			"the codebase. Search before storing to avoid duplicates.",
		tool.Object(map[string]*tool.Property{
			"action":   {Type: "string", Description: "Operation to perform.", Enum: allActions},
			"key":      {Type: "string", Description: "Entry identifier, for update/forget (from search)."},
			"query":    {Type: "string", Description: "For search: case-insensitive text to match."},
			"content":  {Type: "string", Description: "For store/update: the fact to remember."},
			"category": {Type: "string", Description: "Optional grouping; for purge, the category to clear."},
			"limit":    {Type: "integer", Description: "For search: max results."},
			"ttl":      {Type: "string", Description: "For store: lifetime as a Go duration (e.g. '720h'); omit for permanent."},
			"metadata": {Type: "object", Description: "Optional structured metadata to attach."},
		}, "action"),
		func(ctx context.Context, args Args) (tool.Result, error) {
			return run(ctx, store, args)
		},
	), nil
}

func run(ctx context.Context, store Store, args Args) (tool.Result, error) {
	switch Action(strings.TrimSpace(args.Action)) {
	case ActionStore:
		return runStore(ctx, store, args)
	case ActionSearch:
		return runSearch(ctx, store, args)
	case ActionUpdate:
		return runUpdate(ctx, store, args)
	case ActionForget:
		return runForget(ctx, store, args)
	case ActionPurge:
		return runPurge(ctx, store, args)
	case ActionExport:
		return runExport(ctx, store)
	case "":
		return tool.Result{}, fmt.Errorf("action is required (one of: %s)", strings.Join(allActions, ", "))
	default:
		return tool.Result{}, fmt.Errorf("unknown action %q (want: %s)", args.Action, strings.Join(allActions, ", "))
	}
}

func runStore(ctx context.Context, store Store, args Args) (tool.Result, error) {
	if strings.TrimSpace(args.Content) == "" {
		return tool.Result{}, fmt.Errorf("store requires content")
	}
	e := Entry{Key: args.Key, Content: args.Content, Category: args.Category, Metadata: args.Metadata}
	if ttl := strings.TrimSpace(args.TTL); ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return tool.Result{}, fmt.Errorf("invalid ttl %q: %w", ttl, err)
		}
		e.ExpiresAt = time.Now().Add(d)
	}
	saved, err := store.Put(ctx, e)
	if err != nil {
		return tool.Result{}, fmt.Errorf("store memory: %w", err)
	}
	return tool.Result{
		Content: fmt.Sprintf("Remembered (%s): %s", saved.Key, saved.Content),
		Data:    map[string]any{"entry": saved},
	}, nil
}

func runSearch(ctx context.Context, store Store, args Args) (tool.Result, error) {
	entries, err := store.Search(ctx, Query{Text: args.Query, Category: args.Category, Limit: args.Limit})
	if err != nil {
		return tool.Result{}, fmt.Errorf("search memory: %w", err)
	}
	return tool.Result{Content: formatEntries(entries, "No matching memories."), Data: map[string]any{"entries": entries, "count": len(entries)}}, nil
}

func runUpdate(ctx context.Context, store Store, args Args) (tool.Result, error) {
	if strings.TrimSpace(args.Key) == "" {
		return tool.Result{}, fmt.Errorf("update requires key")
	}
	if args.Content == "" && args.Category == "" && args.Metadata == nil {
		return tool.Result{}, fmt.Errorf("update requires new content, category, or metadata")
	}
	saved, err := store.Update(ctx, args.Key, Entry{Content: args.Content, Category: args.Category, Metadata: args.Metadata})
	if err != nil {
		return tool.Result{}, fmt.Errorf("update memory: %w", err)
	}
	return tool.Result{Content: fmt.Sprintf("Updated (%s): %s", saved.Key, saved.Content), Data: map[string]any{"entry": saved}}, nil
}

func runForget(ctx context.Context, store Store, args Args) (tool.Result, error) {
	if strings.TrimSpace(args.Key) == "" {
		return tool.Result{}, fmt.Errorf("forget requires key")
	}
	if err := store.Delete(ctx, args.Key); err != nil {
		return tool.Result{}, fmt.Errorf("forget memory: %w", err)
	}
	return tool.Result{Content: "Forgot " + args.Key + ".", Data: map[string]any{"forgot": args.Key}}, nil
}

func runPurge(ctx context.Context, store Store, args Args) (tool.Result, error) {
	n, err := store.Purge(ctx, args.Category)
	if err != nil {
		return tool.Result{}, fmt.Errorf("purge memory: %w", err)
	}
	scope := "all memories"
	if args.Category != "" {
		scope = fmt.Sprintf("category %q", args.Category)
	}
	return tool.Result{Content: fmt.Sprintf("Purged %d entries (%s).", n, scope), Data: map[string]any{"purged": n}}, nil
}

func runExport(ctx context.Context, store Store) (tool.Result, error) {
	entries, err := store.Search(ctx, Query{})
	if err != nil {
		return tool.Result{}, fmt.Errorf("export memory: %w", err)
	}
	return tool.Result{Content: formatEntries(entries, "No memories stored."), Data: map[string]any{"entries": entries, "count": len(entries)}}, nil
}

func formatEntries(entries []Entry, empty string) string {
	if len(entries) == 0 {
		return empty
	}
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(e.Key)
		if e.Category != "" {
			fmt.Fprintf(&b, " [%s]", e.Category)
		}
		fmt.Fprintf(&b, ": %s", e.Content)
	}
	return b.String()
}
