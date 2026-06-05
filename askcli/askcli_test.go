package askcli

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sho0pi/agenttools/ask"
)

func TestNew_Answer(t *testing.T) {
	in := strings.NewReader("Postgres\n")
	var out strings.Builder
	fn := New(in, &out)

	got, err := fn(context.Background(), ask.Question{Prompt: "Which DB?"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Postgres" {
		t.Fatalf("got %q, want Postgres", got)
	}
	if !strings.Contains(out.String(), "Which DB?") {
		t.Fatalf("prompt not printed: %q", out.String())
	}
}

func TestNew_EmptyAnswerReturnsDefault(t *testing.T) {
	in := strings.NewReader("\n")
	fn := New(in, &strings.Builder{})

	got, err := fn(context.Background(), ask.Question{Prompt: "x?", Default: "sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "sqlite" {
		t.Fatalf("got %q, want default 'sqlite'", got)
	}
}

func TestNew_OptionsAndContextPrinted(t *testing.T) {
	in := strings.NewReader("1\n")
	var out strings.Builder
	fn := New(in, &out)

	_, _ = fn(context.Background(), ask.Question{
		Prompt:  "Pick one",
		Context: "choose wisely",
		Options: []string{"A", "B", "C"},
		Default: "A",
	})

	printed := out.String()
	if !strings.Contains(printed, "choose wisely") {
		t.Fatalf("context not printed: %q", printed)
	}
	if !strings.Contains(printed, "1) A") || !strings.Contains(printed, "3) C") {
		t.Fatalf("options not printed: %q", printed)
	}
	if !strings.Contains(printed, "[default: A]") {
		t.Fatalf("default not printed: %q", printed)
	}
}

func TestNew_CtxCancel(t *testing.T) {
	// io.Pipe blocks until the writer sends — simulates a user who never types.
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	fn := New(pr, &strings.Builder{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	got, err := fn(ctx, ask.Question{Prompt: "x?", Default: "fallback"})
	if err == nil {
		t.Fatal("expected context error")
	}
	if got != "fallback" {
		t.Fatalf("got %q, want 'fallback'", got)
	}
}
