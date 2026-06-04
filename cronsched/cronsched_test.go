package cronsched

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sho0pi/agenttools/cron"
)

func newSched(t *testing.T, run func(context.Context, cron.Job)) *Scheduler {
	t.Helper()
	if run == nil {
		run = func(context.Context, cron.Job) {}
	}
	s, err := New(Config{Run: run})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	return s
}

func TestNew_NilRun(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected error for nil Run callback")
	}
}

func TestScheduler_CRUD(t *testing.T) {
	ctx := context.Background()
	s := newSched(t, nil)

	// Add (long interval so it never fires during the test).
	j, err := s.Add(ctx, "24h", "say hi")
	if err != nil {
		t.Fatal(err)
	}
	if j.ID == "" || !j.Enabled || j.Next.IsZero() {
		t.Fatalf("Add returned %+v", j)
	}

	// cron expression is accepted
	if _, err := s.Add(ctx, "0 9 * * *", "morning"); err != nil {
		t.Fatalf("cron expr add: %v", err)
	}
	// invalid schedule rejected
	if _, err := s.Add(ctx, "not a schedule", "x"); err == nil {
		t.Fatal("invalid schedule should error")
	}

	// List
	jobs, _ := s.List(ctx)
	if len(jobs) != 2 {
		t.Fatalf("List = %d jobs, want 2", len(jobs))
	}

	// Get
	got, err := s.Get(ctx, j.ID)
	if err != nil || got.Instruction != "say hi" {
		t.Fatalf("Get = %+v, err %v", got, err)
	}
	if _, err := s.Get(ctx, "missing"); err == nil {
		t.Fatal("Get unknown id should error")
	}

	// Update
	up, err := s.Update(ctx, j.ID, "12h", "say bye")
	if err != nil {
		t.Fatal(err)
	}
	if up.Schedule != "12h" || up.Instruction != "say bye" {
		t.Fatalf("Update = %+v", up)
	}

	// Remove
	if err := s.Remove(ctx, j.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, j.ID); err == nil {
		t.Fatal("removed job should be gone")
	}
}

func TestScheduler_SetEnabled(t *testing.T) {
	ctx := context.Background()
	s := newSched(t, nil)
	j, _ := s.Add(ctx, "24h", "x")

	dis, err := s.SetEnabled(ctx, j.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if dis.Enabled || !dis.Next.IsZero() {
		t.Fatalf("disabled job = %+v (Next should be zero)", dis)
	}

	en, err := s.SetEnabled(ctx, j.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !en.Enabled || en.Next.IsZero() {
		t.Fatalf("re-enabled job = %+v (Next should be set)", en)
	}
}

func TestScheduler_Fires(t *testing.T) {
	fired := make(chan cron.Job, 1)
	s := newSched(t, func(_ context.Context, j cron.Job) {
		select {
		case fired <- j:
		default:
		}
	})
	if _, err := s.Add(context.Background(), "30ms", "tick"); err != nil {
		t.Fatal(err)
	}
	select {
	case j := <-fired:
		if j.Instruction != "tick" {
			t.Fatalf("fired job = %+v", j)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("job did not fire within 2s")
	}
}

// TestWithCronTool wires the gocron scheduler into the real cron tool, proving
// it satisfies the provider seam end to end.
func TestWithCronTool(t *testing.T) {
	s := newSched(t, nil)
	tr, err := cron.New(s)
	if err != nil {
		t.Fatal(err)
	}

	exec := func(args cron.Args) (string, error) {
		raw, _ := json.Marshal(args)
		res, err := tr.Execute(context.Background(), raw)
		return res.Content, err
	}

	if _, err := exec(cron.Args{Action: "add", Schedule: "24h", Instruction: "daily report"}); err != nil {
		t.Fatal(err)
	}
	listed, err := exec(cron.Args{Action: "list"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed, "daily report") {
		t.Fatalf("list via tool = %q", listed)
	}
}
