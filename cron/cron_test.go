package cron

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sho0pi/agenttools/tool"
)

// fakeScheduler is an in-memory Scheduler for tests. Hooks let a case force
// errors without a separate type per case.
type fakeScheduler struct {
	jobs    []Job
	nextID  int
	addErr  error
	listErr error
	rmErr   error
}

func (f *fakeScheduler) find(id string) (*Job, bool) {
	for i := range f.jobs {
		if f.jobs[i].ID == id {
			return &f.jobs[i], true
		}
	}
	return nil, false
}

func (f *fakeScheduler) Get(_ context.Context, id string) (Job, error) {
	if j, ok := f.find(id); ok {
		return *j, nil
	}
	return Job{}, errors.New("not found")
}

func (f *fakeScheduler) Update(_ context.Context, id, schedule, instruction string) (Job, error) {
	j, ok := f.find(id)
	if !ok {
		return Job{}, errors.New("not found")
	}
	if schedule != "" {
		j.Schedule = schedule
	}
	if instruction != "" {
		j.Instruction = instruction
	}
	return *j, nil
}

func (f *fakeScheduler) SetEnabled(_ context.Context, id string, enabled bool) (Job, error) {
	j, ok := f.find(id)
	if !ok {
		return Job{}, errors.New("not found")
	}
	j.Enabled = enabled
	return *j, nil
}

func (f *fakeScheduler) Add(_ context.Context, schedule, instruction string) (Job, error) {
	if f.addErr != nil {
		return Job{}, f.addErr
	}
	f.nextID++
	j := Job{
		ID:          strings.TrimSpace(string(rune('0' + f.nextID))),
		Schedule:    schedule,
		Instruction: instruction,
		Enabled:     true,
	}
	f.jobs = append(f.jobs, j)
	return j, nil
}

func (f *fakeScheduler) List(_ context.Context) ([]Job, error) {
	return f.jobs, f.listErr
}

func (f *fakeScheduler) Remove(_ context.Context, id string) error {
	if f.rmErr != nil {
		return f.rmErr
	}
	for i, j := range f.jobs {
		if j.ID == id {
			f.jobs = append(f.jobs[:i], f.jobs[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

func callTool(t *testing.T, tr tool.Tool, args Args) (tool.Result, error) {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return tr.Execute(context.Background(), raw)
}

func mustNew(t *testing.T, sched Scheduler) tool.Tool {
	t.Helper()
	tr, err := New(sched)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tr
}

func TestCron_NilScheduler(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil scheduler")
	}
}

func TestCron_Add(t *testing.T) {
	tests := []struct {
		name    string
		args    Args
		wantErr bool
		wantSub string // substring expected in Content on success
	}{
		{
			name:    "duration schedule",
			args:    Args{Action: "add", Schedule: "1h", Instruction: "Tell the date."},
			wantSub: "1h",
		},
		{
			name:    "cron expression",
			args:    Args{Action: "add", Schedule: "0 9 * * *", Instruction: "Morning report."},
			wantSub: "0 9 * * *",
		},
		{
			name:    "missing instruction",
			args:    Args{Action: "add", Schedule: "1h"},
			wantErr: true,
		},
		{
			name:    "missing schedule",
			args:    Args{Action: "add", Instruction: "x"},
			wantErr: true,
		},
		{
			name:    "blank schedule is trimmed to empty",
			args:    Args{Action: "add", Schedule: "   ", Instruction: "x"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := mustNew(t, &fakeScheduler{})
			res, err := callTool(t, tr, tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(res.Content, tc.wantSub) {
				t.Fatalf("content = %q, want substring %q", res.Content, tc.wantSub)
			}
		})
	}
}

func TestCron_AddPropagatesSchedulerError(t *testing.T) {
	tr := mustNew(t, &fakeScheduler{addErr: errors.New("bad cron expr")})
	_, err := callTool(t, tr, Args{Action: "add", Schedule: "nonsense", Instruction: "x"})
	if err == nil || !strings.Contains(err.Error(), "bad cron expr") {
		t.Fatalf("want wrapped scheduler error, got %v", err)
	}
}

func TestCron_List(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		tr := mustNew(t, &fakeScheduler{})
		res, err := callTool(t, tr, Args{Action: "list"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(res.Content, "No scheduled jobs") {
			t.Fatalf("content = %q", res.Content)
		}
	})

	t.Run("with jobs and next time", func(t *testing.T) {
		next := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)
		f := &fakeScheduler{jobs: []Job{
			{ID: "1", Schedule: "0 9 * * *", Instruction: "report", Enabled: true, Next: next},
			{ID: "2", Schedule: "1h", Instruction: "ping", Enabled: false},
		}}
		res, err := callTool(t, mustNew(t, f), Args{Action: "list"})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"1", "report", "enabled", "2", "ping", "disabled", "2026-06-05T09:00:00Z"} {
			if !strings.Contains(res.Content, want) {
				t.Fatalf("content missing %q:\n%s", want, res.Content)
			}
		}
		if res.Data["count"] != 2 {
			t.Fatalf("count = %v, want 2", res.Data["count"])
		}
	})
}

func TestCron_Remove(t *testing.T) {
	f := &fakeScheduler{jobs: []Job{{ID: "1", Schedule: "1h", Instruction: "x", Enabled: true}}}
	tr := mustNew(t, f)

	if _, err := callTool(t, tr, Args{Action: "remove"}); err == nil {
		t.Fatal("expected error for missing job_id")
	}

	res, err := callTool(t, tr, Args{Action: "remove", JobID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "Removed job 1") {
		t.Fatalf("content = %q", res.Content)
	}
	if len(f.jobs) != 0 {
		t.Fatalf("job not removed: %v", f.jobs)
	}

	if _, err := callTool(t, tr, Args{Action: "remove", JobID: "missing"}); err == nil {
		t.Fatal("expected error removing unknown job")
	}
}

func TestCron_Get(t *testing.T) {
	f := &fakeScheduler{jobs: []Job{{ID: "1", Schedule: "1h", Instruction: "ping", Enabled: true}}}
	tr := mustNew(t, f)

	if _, err := callTool(t, tr, Args{Action: "get"}); err == nil {
		t.Fatal("expected error for missing job_id")
	}

	res, err := callTool(t, tr, Args{Action: "get", JobID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "ping") {
		t.Fatalf("content = %q", res.Content)
	}

	if _, err := callTool(t, tr, Args{Action: "get", JobID: "missing"}); err == nil {
		t.Fatal("expected error for unknown job")
	}
}

func TestCron_Update(t *testing.T) {
	f := &fakeScheduler{jobs: []Job{{ID: "1", Schedule: "1h", Instruction: "old", Enabled: true}}}
	tr := mustNew(t, f)

	// No new fields → error.
	if _, err := callTool(t, tr, Args{Action: "update", JobID: "1"}); err == nil {
		t.Fatal("expected error when nothing to update")
	}
	// Missing job_id → error.
	if _, err := callTool(t, tr, Args{Action: "update", Schedule: "2h"}); err == nil {
		t.Fatal("expected error for missing job_id")
	}

	// Patch instruction only; schedule must stay.
	res, err := callTool(t, tr, Args{Action: "update", JobID: "1", Instruction: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "new") || !strings.Contains(res.Content, "1h") {
		t.Fatalf("content = %q, want updated instruction and unchanged schedule", res.Content)
	}
	if j, _ := f.find("1"); j.Instruction != "new" || j.Schedule != "1h" {
		t.Fatalf("job not patched correctly: %+v", *j)
	}
}

func TestCron_EnableDisable(t *testing.T) {
	f := &fakeScheduler{jobs: []Job{{ID: "1", Schedule: "1h", Instruction: "x", Enabled: true}}}
	tr := mustNew(t, f)

	res, err := callTool(t, tr, Args{Action: "disable", JobID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "Disabled job 1") {
		t.Fatalf("content = %q", res.Content)
	}
	if j, _ := f.find("1"); j.Enabled {
		t.Fatal("job still enabled after disable")
	}

	res, err = callTool(t, tr, Args{Action: "enable", JobID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "Enabled job 1") {
		t.Fatalf("content = %q", res.Content)
	}
	if j, _ := f.find("1"); !j.Enabled {
		t.Fatal("job still disabled after enable")
	}

	// Both require job_id.
	for _, a := range []string{"enable", "disable"} {
		if _, err := callTool(t, tr, Args{Action: a}); err == nil {
			t.Fatalf("%s: expected error for missing job_id", a)
		}
	}
}

func TestCron_ActionValidation(t *testing.T) {
	tr := mustNew(t, &fakeScheduler{})
	for _, action := range []string{"", "delete", "ADD", "unknown"} {
		if _, err := callTool(t, tr, Args{Action: action}); err == nil {
			t.Fatalf("action %q: want error, got nil", action)
		}
	}
}
