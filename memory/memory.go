// Package memory provides the memory tool and the Store provider it delegates
// to. The tool lets an agent persist and recall durable facts across sessions
// (preferences, project decisions, stable context). The Store owns persistence
// and retrieval; the default InMemoryStore keeps entries in process, while
// production deployments back it with SQLite, Redis, Postgres, or a vector DB.
package memory

import (
	"context"
	"time"
)

// Entry is one stored memory.
type Entry struct {
	Key       string         `json:"key"`                // unique identifier (generated if empty on Put)
	Content   string         `json:"content"`            // the fact itself
	Category  string         `json:"category,omitempty"` // optional grouping (e.g. "preference")
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitzero"`
	ExpiresAt time.Time      `json:"expires_at,omitzero"` // zero → never expires
}

// Query filters a Search.
type Query struct {
	Text     string // case-insensitive substring match on Content (empty → match all)
	Category string // restrict to this category (empty → any)
	Limit    int    // cap results (0 → no cap)
}

// Store persists and retrieves memory entries. Implementations own storage,
// expiry, and (optionally) tenant isolation. All methods are safe for
// concurrent use.
type Store interface {
	// Put stores e and returns the stored entry (with its key and timestamps
	// populated). If e.Key is empty, the implementation assigns one.
	Put(ctx context.Context, e Entry) (Entry, error)
	// Search returns entries matching q.
	Search(ctx context.Context, q Query) ([]Entry, error)
	// Update replaces the content/category/metadata of the entry with key,
	// returning the updated entry. Unknown key is an error.
	Update(ctx context.Context, key string, e Entry) (Entry, error)
	// Delete removes the entry with key. Unknown key is an error.
	Delete(ctx context.Context, key string) error
	// Purge removes all entries in category, returning the count removed. An
	// empty category purges everything.
	Purge(ctx context.Context, category string) (int, error)
}
