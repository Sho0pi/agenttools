package askim

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sho0pi/agenttools/ask"
)

func noopSender(_ context.Context, _ ask.Question) error { return nil }

func TestAsk_Answer(t *testing.T) {
	p := New(noopSender)
	fn := p.Ask("chat1")

	go func() {
		time.Sleep(10 * time.Millisecond)
		p.Reply("chat1", "Postgres")
	}()

	got, err := fn(context.Background(), ask.Question{Prompt: "Which DB?"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Postgres" {
		t.Fatalf("got %q, want Postgres", got)
	}
}

func TestAsk_EmptyReplyReturnsDefault(t *testing.T) {
	p := New(noopSender)
	fn := p.Ask("chat1")

	go func() {
		time.Sleep(10 * time.Millisecond)
		p.Reply("chat1", "")
	}()

	got, err := fn(context.Background(), ask.Question{Prompt: "x?", Default: "sqlite"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "sqlite" {
		t.Fatalf("got %q, want default 'sqlite'", got)
	}
}

func TestAsk_CtxCancel(t *testing.T) {
	p := New(noopSender)
	fn := p.Ask("chat1")

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

func TestAsk_SenderError(t *testing.T) {
	p := New(func(_ context.Context, _ ask.Question) error {
		return errors.New("send failed")
	})
	fn := p.Ask("chat1")

	_, err := fn(context.Background(), ask.Question{Prompt: "x?"})
	if err == nil {
		t.Fatal("sender error should propagate")
	}
}

func TestReply_NoPendingAsk(t *testing.T) {
	p := New(noopSender)
	// Must not panic or deadlock.
	p.Reply("nobody", "hello")
}

func TestAsk_ConcurrentDifferentIDs(t *testing.T) {
	p := New(noopSender)
	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		id := string(rune('A' + i))
		go func() {
			defer wg.Done()
			fn := p.Ask(id)
			go func() {
				time.Sleep(5 * time.Millisecond)
				p.Reply(id, "answer-"+id)
			}()
			got, err := fn(context.Background(), ask.Question{Prompt: "q?"})
			if err != nil {
				t.Errorf("id=%s: %v", id, err)
			}
			if got != "answer-"+id {
				t.Errorf("id=%s: got %q", id, got)
			}
		}()
	}
	wg.Wait()
}
