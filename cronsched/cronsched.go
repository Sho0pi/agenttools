// Package cronsched implements the cron.Scheduler provider using gocron
// (github.com/go-co-op/gocron). It parses each job's schedule string — a Go
// duration ("1h") or a 5-field cron expression ("0 9 * * *") — tracks the
// clock, and invokes a caller-supplied Run callback when a job fires. The cron
// tool stays clock-free; this provider owns time.
package cronsched

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"github.com/sho0pi/agenttools/cron"
)

// Config configures a Scheduler.
type Config struct {
	// Run is invoked each time a job fires, with the job that fired. It is
	// required — typically it hands the job's Instruction to the agent loop.
	Run func(ctx context.Context, job cron.Job)
	// BaseContext is passed to Run on each fire (default context.Background()).
	BaseContext context.Context
}

// record is the provider's bookkeeping for one scheduled job. The gocron job
// handle is nil while the job is disabled.
type record struct {
	id          string
	schedule    string
	instruction string
	enabled     bool
	job         gocron.Job
}

// Scheduler is a gocron-backed cron.Scheduler. It is safe for concurrent use.
type Scheduler struct {
	gs      gocron.Scheduler
	run     func(ctx context.Context, job cron.Job)
	baseCtx context.Context

	mu      sync.Mutex
	records map[string]*record
}

// compile-time assurance that Scheduler satisfies the tool's provider seam.
var _ cron.Scheduler = (*Scheduler)(nil)

// New builds and starts a Scheduler, or returns an error if the Run callback is
// nil or gocron cannot start. Call Stop to release resources.
func New(cfg Config) (*Scheduler, error) {
	if cfg.Run == nil {
		return nil, fmt.Errorf("cronsched: Run callback must not be nil")
	}
	gs, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("new gocron scheduler: %w", err)
	}
	base := cfg.BaseContext
	if base == nil {
		base = context.Background()
	}
	s := &Scheduler{gs: gs, run: cfg.Run, baseCtx: base, records: make(map[string]*record)}
	gs.Start()
	return s, nil
}

// Stop shuts down the underlying scheduler.
func (s *Scheduler) Stop() error { return s.gs.Shutdown() }

// Add implements cron.Scheduler.
func (s *Scheduler) Add(_ context.Context, schedule, instruction string) (cron.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := &record{id: uuid.NewString(), schedule: schedule, instruction: instruction, enabled: true}
	if err := s.scheduleLocked(r); err != nil {
		return cron.Job{}, err
	}
	s.records[r.id] = r
	return s.toJob(r), nil
}

// List implements cron.Scheduler.
func (s *Scheduler) List(_ context.Context) ([]cron.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]cron.Job, 0, len(s.records))
	for _, r := range s.records {
		out = append(out, s.toJob(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Get implements cron.Scheduler.
func (s *Scheduler) Get(_ context.Context, id string) (cron.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	if !ok {
		return cron.Job{}, fmt.Errorf("no job with id %q", id)
	}
	return s.toJob(r), nil
}

// Update implements cron.Scheduler. Empty schedule/instruction leave that field
// unchanged.
func (s *Scheduler) Update(_ context.Context, id, schedule, instruction string) (cron.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.records[id]
	if !ok {
		return cron.Job{}, fmt.Errorf("no job with id %q", id)
	}
	if schedule != "" {
		r.schedule = schedule
	}
	if instruction != "" {
		r.instruction = instruction
	}
	// Reschedule so a changed schedule takes effect.
	if r.enabled {
		s.removeLocked(r)
		if err := s.scheduleLocked(r); err != nil {
			return cron.Job{}, err
		}
	}
	return s.toJob(r), nil
}

// Remove implements cron.Scheduler.
func (s *Scheduler) Remove(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	if !ok {
		return fmt.Errorf("no job with id %q", id)
	}
	s.removeLocked(r)
	delete(s.records, id)
	return nil
}

// SetEnabled implements cron.Scheduler.
func (s *Scheduler) SetEnabled(_ context.Context, id string, enabled bool) (cron.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.records[id]
	if !ok {
		return cron.Job{}, fmt.Errorf("no job with id %q", id)
	}
	if enabled == r.enabled {
		return s.toJob(r), nil
	}
	if enabled {
		r.enabled = true
		if err := s.scheduleLocked(r); err != nil {
			r.enabled = false
			return cron.Job{}, err
		}
	} else {
		s.removeLocked(r)
		r.enabled = false
	}
	return s.toJob(r), nil
}

// scheduleLocked creates the gocron job for r. Caller holds s.mu.
func (s *Scheduler) scheduleLocked(r *record) error {
	def, err := jobDefinition(r.schedule)
	if err != nil {
		return err
	}
	id := r.id
	job, err := s.gs.NewJob(def, gocron.NewTask(func() { s.fire(id) }))
	if err != nil {
		return fmt.Errorf("schedule %q: %w", r.schedule, err)
	}
	r.job = job
	return nil
}

// removeLocked detaches r's gocron job if present. Caller holds s.mu.
func (s *Scheduler) removeLocked(r *record) {
	if r.job != nil {
		_ = s.gs.RemoveJob(r.job.ID())
		r.job = nil
	}
}

// fire is the gocron task body: it looks up the record and runs the callback.
func (s *Scheduler) fire(id string) {
	s.mu.Lock()
	r, ok := s.records[id]
	if !ok || !r.enabled {
		s.mu.Unlock()
		return
	}
	job := s.toJob(r)
	s.mu.Unlock()
	s.run(s.baseCtx, job)
}

// toJob builds the tool-facing cron.Job from a record. Caller holds s.mu.
func (s *Scheduler) toJob(r *record) cron.Job {
	j := cron.Job{ID: r.id, Schedule: r.schedule, Instruction: r.instruction, Enabled: r.enabled}
	if r.enabled && r.job != nil {
		if next, err := r.job.NextRun(); err == nil {
			j.Next = next
		}
	}
	return j
}

// jobDefinition turns a schedule string into a gocron JobDefinition. A value
// that parses as a Go duration becomes a DurationJob; otherwise it is treated as
// a 5-field cron expression (validated by gocron when the job is created).
func jobDefinition(schedule string) (gocron.JobDefinition, error) {
	if d, err := time.ParseDuration(schedule); err == nil {
		if d <= 0 {
			return nil, fmt.Errorf("duration must be positive: %q", schedule)
		}
		return gocron.DurationJob(d), nil
	}
	return gocron.CronJob(schedule, false), nil
}
