package memory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// InMemoryStore is the default Store: an in-process, concurrency-safe map of
// entries. It is ideal for tests and single-process agents; it does not persist
// across restarts. Expired entries are skipped on read and dropped lazily.
type InMemoryStore struct {
	mu    sync.RWMutex
	now   func() time.Time // injectable clock for tests
	seq   int
	byKey map[string]Entry
}

// NewInMemoryStore returns an empty InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{now: time.Now, byKey: make(map[string]Entry)}
}

// Put implements Store.
func (s *InMemoryStore) Put(_ context.Context, e Entry) (Entry, error) {
	if strings.TrimSpace(e.Content) == "" {
		return Entry{}, fmt.Errorf("content is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if e.Key == "" {
		s.seq++
		e.Key = "mem-" + strconv.Itoa(s.seq)
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = s.now()
	}
	s.byKey[e.Key] = e
	return e, nil
}

// Search implements Store.
func (s *InMemoryStore) Search(_ context.Context, q Query) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	needle := strings.ToLower(strings.TrimSpace(q.Text))
	var out []Entry
	for _, e := range s.byKey {
		if s.expired(e) {
			continue
		}
		if q.Category != "" && e.Category != q.Category {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(e.Content), needle) {
			continue
		}
		out = append(out, e)
	}
	// Newest first for stable, useful ordering.
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// Update implements Store.
func (s *InMemoryStore) Update(_ context.Context, key string, patch Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.byKey[key]
	if !ok || s.expired(e) {
		return Entry{}, fmt.Errorf("no memory with key %q", key)
	}
	if patch.Content != "" {
		e.Content = patch.Content
	}
	if patch.Category != "" {
		e.Category = patch.Category
	}
	if patch.Metadata != nil {
		e.Metadata = patch.Metadata
	}
	if !patch.ExpiresAt.IsZero() {
		e.ExpiresAt = patch.ExpiresAt
	}
	s.byKey[key] = e
	return e, nil
}

// Delete implements Store.
func (s *InMemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byKey[key]; !ok {
		return fmt.Errorf("no memory with key %q", key)
	}
	delete(s.byKey, key)
	return nil
}

// Purge implements Store.
func (s *InMemoryStore) Purge(_ context.Context, category string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, e := range s.byKey {
		if category == "" || e.Category == category {
			delete(s.byKey, k)
			n++
		}
	}
	return n, nil
}

func (s *InMemoryStore) expired(e Entry) bool {
	return !e.ExpiresAt.IsZero() && s.now().After(e.ExpiresAt)
}
