package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sho0pi/agenttools/tool"
)

func TestInMemoryStore(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()

	a, err := s.Put(ctx, Entry{Content: "User prefers Go", Category: "preference"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Key == "" || a.CreatedAt.IsZero() {
		t.Fatalf("Put did not populate key/time: %+v", a)
	}
	if _, err := s.Put(ctx, Entry{Content: "Deploy to staging first", Category: "process"}); err != nil {
		t.Fatal(err)
	}

	// content required
	if _, err := s.Put(ctx, Entry{Content: "  "}); err == nil {
		t.Fatal("blank content should error")
	}

	// search by text
	got, _ := s.Search(ctx, Query{Text: "go"})
	if len(got) != 1 || got[0].Content != "User prefers Go" {
		t.Fatalf("text search = %+v", got)
	}
	// search by category
	got, _ = s.Search(ctx, Query{Category: "process"})
	if len(got) != 1 {
		t.Fatalf("category search = %+v", got)
	}
	// search all
	if all, _ := s.Search(ctx, Query{}); len(all) != 2 {
		t.Fatalf("search all = %d, want 2", len(all))
	}

	// update
	if _, err := s.Update(ctx, a.Key, Entry{Content: "User loves Go"}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Search(ctx, Query{Text: "loves"})
	if len(got) != 1 {
		t.Fatal("update not reflected in search")
	}
	if _, err := s.Update(ctx, "missing", Entry{Content: "x"}); err == nil {
		t.Fatal("update of unknown key should error")
	}

	// delete
	if err := s.Delete(ctx, a.Key); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, a.Key); err == nil {
		t.Fatal("double delete should error")
	}

	// purge
	if n, _ := s.Purge(ctx, ""); n != 1 {
		t.Fatalf("purge all removed %d, want 1", n)
	}
}

func TestInMemoryStore_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryStore()
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }

	if _, err := s.Put(ctx, Entry{Content: "ephemeral", ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Search(ctx, Query{}); len(got) != 1 {
		t.Fatalf("entry should be live before expiry: %+v", got)
	}
	// advance past expiry
	now = now.Add(2 * time.Hour)
	if got, _ := s.Search(ctx, Query{}); len(got) != 0 {
		t.Fatalf("expired entry should be hidden: %+v", got)
	}
}

func TestMemoryTool(t *testing.T) {
	tr, err := New(NewInMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	do := func(args Args) (tool.Result, error) {
		raw, _ := json.Marshal(args)
		return tr.Execute(context.Background(), raw)
	}

	// store
	res, err := do(Args{Action: "store", Content: "User prefers TypeScript", Category: "preference"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "Remembered") {
		t.Fatalf("store content = %q", res.Content)
	}

	// search finds it
	res, _ = do(Args{Action: "search", Query: "typescript"})
	if !strings.Contains(res.Content, "TypeScript") {
		t.Fatalf("search content = %q", res.Content)
	}
	if res.Data["count"] != 1 {
		t.Fatalf("search count = %v", res.Data["count"])
	}

	// export
	res, _ = do(Args{Action: "export"})
	if !strings.Contains(res.Content, "TypeScript") {
		t.Fatalf("export content = %q", res.Content)
	}

	// validation
	if _, err := do(Args{Action: "store"}); err == nil {
		t.Fatal("store without content should error")
	}
	if _, err := do(Args{Action: "update"}); err == nil {
		t.Fatal("update without key should error")
	}
	if _, err := do(Args{Action: "store", Content: "x", TTL: "bogus"}); err == nil {
		t.Fatal("invalid ttl should error")
	}
	if _, err := do(Args{Action: "bogus"}); err == nil {
		t.Fatal("unknown action should error")
	}
	if _, err := do(Args{}); err == nil {
		t.Fatal("missing action should error")
	}

	// purge
	res, _ = do(Args{Action: "purge"})
	if !strings.Contains(res.Content, "Purged") {
		t.Fatalf("purge content = %q", res.Content)
	}
}

func TestNew_NilStore(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil Store")
	}
}
