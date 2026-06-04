package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// Registry holds tools by name and dispatches calls to them.
// All methods are safe for concurrent use.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds t. A later registration with the same name replaces the earlier one.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Tools returns every registered tool in lexicographic name order.
func (r *Registry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// FilteredTools returns tools whose names appear in allowed, in the order given.
// An empty allowed list returns all tools (lexicographic order).
func (r *Registry) FilteredTools(allowed []string) []Tool {
	if len(allowed) == 0 {
		return r.Tools()
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(allowed))
	for _, name := range allowed {
		if t, ok := r.tools[name]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Dispatch runs the named tool. args is re-marshalled to JSON so the tool can
// decode into its own typed struct — this handles the map[string]any that
// provider SDKs return after decoding a tool-call response.
func (r *Registry) Dispatch(ctx context.Context, name string, args map[string]any) (Result, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("unknown tool %q", name)
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return Result{}, fmt.Errorf("marshal args for %q: %w", name, err)
	}
	return t.Execute(ctx, raw)
}
