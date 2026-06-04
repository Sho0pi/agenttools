package search

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sho0pi/agenttools/tool"
)

func callTool(t *testing.T, tr tool.Tool, raw string) (tool.Result, error) {
	t.Helper()
	return tr.Execute(context.Background(), json.RawMessage(raw))
}

func mustNew(t *testing.T, p Provider) tool.Tool {
	t.Helper()
	tr, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tr
}

func TestWebSearch_HappyPath(t *testing.T) {
	var gotMax int
	tr := mustNew(t, func(_ context.Context, _ string, max int) (string, error) {
		gotMax = max
		return "1. Title — https://x.test", nil
	})
	res, err := callTool(t, tr, `{"query":"go frameworks"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "https://x.test") {
		t.Fatalf("content = %q", res.Content)
	}
	if gotMax != defaultResults {
		t.Fatalf("max = %d, want default %d", gotMax, defaultResults)
	}
}

func TestWebSearch_NilProvider(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestWebSearch_EmptyQuery(t *testing.T) {
	tr := mustNew(t, func(_ context.Context, _ string, _ int) (string, error) { return "x", nil })
	if _, err := callTool(t, tr, `{"query":"   "}`); err == nil {
		t.Fatal("expected error for blank query")
	}
}

func TestWebSearch_MaxResultsClamped(t *testing.T) {
	var gotMax int
	tr := mustNew(t, func(_ context.Context, _ string, max int) (string, error) {
		gotMax = max
		return "x", nil
	})
	if _, err := callTool(t, tr, `{"query":"q","max_results":999}`); err != nil {
		t.Fatal(err)
	}
	if gotMax != maxResults {
		t.Fatalf("max = %d, want clamp to %d", gotMax, maxResults)
	}
}

func TestWebSearch_NoResults(t *testing.T) {
	tr := mustNew(t, func(_ context.Context, _ string, _ int) (string, error) { return "  \n ", nil })
	res, err := callTool(t, tr, `{"query":"obscure"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "No results") {
		t.Fatalf("content = %q, want no-results message", res.Content)
	}
}

func TestWebSearch_ProviderError(t *testing.T) {
	tr := mustNew(t, func(_ context.Context, _ string, _ int) (string, error) {
		return "", errors.New("ddg down")
	})
	if _, err := callTool(t, tr, `{"query":"q"}`); err == nil {
		t.Fatal("expected provider error to propagate")
	}
}
