// Package cron provides the cron tool: the model schedules, lists, and removes
// recurring instructions that an agent runs later. The tool is provider-neutral
// — it never touches the clock itself. A Scheduler implementation (a future one
// backed by gocron) owns parsing the schedule, tracking time, and firing jobs.
package cron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sho0pi/agenttools/tool"
)

// Action names the cron operation the model requested.
type Action string

const (
	ActionAdd     Action = "add"
	ActionList    Action = "list"
	ActionGet     Action = "get"
	ActionUpdate  Action = "update"
	ActionRemove  Action = "remove"
	ActionEnable  Action = "enable"
	ActionDisable Action = "disable"
)

// allActions lists every supported action in schema/help order.
var allActions = []string{
	string(ActionAdd), string(ActionList), string(ActionGet),
	string(ActionUpdate), string(ActionRemove), string(ActionEnable), string(ActionDisable),
}

// Job is a scheduled instruction. Time fields (Next) are populated by the
// Scheduler; the tool never computes them so the clock stays behind the
// provider seam.
type Job struct {
	ID          string    `json:"id"`
	Schedule    string    `json:"schedule"`    // Go duration ("1m","1h") or 5-field cron expr ("0 9 * * *")
	Instruction string    `json:"instruction"` // what the agent runs each time
	Next        time.Time `json:"next,omitzero"`
	Enabled     bool      `json:"enabled"`
}

// Scheduler is the provider the cron tool drives. It is the single seam where
// time lives: a real implementation (e.g. gocron-backed) parses the schedule
// string, tracks the clock, and fires jobs. The tool only translates model
// arguments into these calls, so it can be tested with a fake and stays neutral
// across providers.
type Scheduler interface {
	// Add registers a job. schedule is a Go duration or a 5-field cron
	// expression; the implementation parses it and returns a wrapped error if
	// it cannot.
	Add(ctx context.Context, schedule, instruction string) (Job, error)
	// List returns all currently scheduled jobs.
	List(ctx context.Context) ([]Job, error)
	// Get returns a single job by id, or an error if it does not exist.
	Get(ctx context.Context, id string) (Job, error)
	// Update patches a job's schedule and/or instruction. An empty string
	// leaves that field unchanged. It returns the updated job.
	Update(ctx context.Context, id, schedule, instruction string) (Job, error)
	// Remove deletes the job with the given id. Removing an unknown id returns
	// an error.
	Remove(ctx context.Context, id string) error
	// SetEnabled enables or disables a job (pausing it without deleting it). It
	// returns the updated job.
	SetEnabled(ctx context.Context, id string, enabled bool) (Job, error)
}

// Args are the cron tool arguments. Which fields are required depends on Action.
type Args struct {
	Action      string `json:"action"`
	Schedule    string `json:"schedule"`
	Instruction string `json:"instruction"`
	JobID       string `json:"job_id"`
}

// New returns the cron tool backed by sched, or an error if sched is nil.
func New(sched Scheduler) (tool.Tool, error) {
	if sched == nil {
		return nil, fmt.Errorf("cron: scheduler must not be nil")
	}
	return tool.NewTypedTool(
		"cron",
		"Schedule and manage recurring instructions that run later without the user asking again. "+
			"action is one of: 'add' (needs schedule + instruction), 'list', 'get' (needs job_id), "+
			"'update' (needs job_id plus a new schedule and/or instruction), 'remove' (needs job_id), "+
			"'enable'/'disable' (need job_id; pause without deleting). "+
			"schedule is a Go duration for 'every X' (e.g. '1m', '30m', '1h', '24h') or a 5-field cron "+
			"expression (e.g. '0 9 * * *' = 9am daily). instruction is what to do each time, phrased as a "+
			"directive (e.g. 'Tell the user today's date.').",
		schema(),
		func(ctx context.Context, args Args) (tool.Result, error) {
			return run(ctx, sched, args)
		},
	), nil
}

func schema() *tool.Schema {
	return tool.Object(map[string]*tool.Property{
		"action": {
			Type:        "string",
			Description: "Operation to perform.",
			Enum:        allActions,
		},
		"schedule": {
			Type:        "string",
			Description: "For 'add'/'update': Go duration ('1m','1h') for every-X, or a cron expression ('0 9 * * *').",
		},
		"instruction": {
			Type:        "string",
			Description: "For 'add'/'update': what to do each time, as a directive.",
		},
		"job_id": {
			Type:        "string",
			Description: "For 'get'/'update'/'remove'/'enable'/'disable': the id of the job (from 'list').",
		},
	}, "action")
}

func run(ctx context.Context, sched Scheduler, args Args) (tool.Result, error) {
	switch Action(strings.TrimSpace(args.Action)) {
	case ActionAdd:
		return runAdd(ctx, sched, args)
	case ActionList:
		return runList(ctx, sched)
	case ActionGet:
		return runGet(ctx, sched, args)
	case ActionUpdate:
		return runUpdate(ctx, sched, args)
	case ActionRemove:
		return runRemove(ctx, sched, args)
	case ActionEnable:
		return runSetEnabled(ctx, sched, args, true)
	case ActionDisable:
		return runSetEnabled(ctx, sched, args, false)
	case "":
		return tool.Result{}, fmt.Errorf("action is required (one of: %s)", strings.Join(allActions, ", "))
	default:
		return tool.Result{}, fmt.Errorf("unknown action %q (want: %s)", args.Action, strings.Join(allActions, ", "))
	}
}

func runAdd(ctx context.Context, sched Scheduler, args Args) (tool.Result, error) {
	schedule := strings.TrimSpace(args.Schedule)
	instruction := strings.TrimSpace(args.Instruction)
	if schedule == "" || instruction == "" {
		return tool.Result{}, fmt.Errorf("add requires both schedule and instruction")
	}

	job, err := sched.Add(ctx, schedule, instruction)
	if err != nil {
		return tool.Result{}, fmt.Errorf("schedule job: %w", err)
	}
	return tool.Result{
		Content: fmt.Sprintf("Scheduled job %s (%s): %s", job.ID, job.Schedule, job.Instruction),
		Data:    map[string]any{"job": job},
	}, nil
}

func runList(ctx context.Context, sched Scheduler) (tool.Result, error) {
	jobs, err := sched.List(ctx)
	if err != nil {
		return tool.Result{}, fmt.Errorf("list jobs: %w", err)
	}
	if len(jobs) == 0 {
		return tool.Result{Content: "No scheduled jobs."}, nil
	}

	var b strings.Builder
	for i, j := range jobs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(formatJob(j))
	}
	return tool.Result{
		Content: b.String(),
		Data:    map[string]any{"jobs": jobs, "count": len(jobs)},
	}, nil
}

// formatJob renders one job as a single human-readable line.
func formatJob(j Job) string {
	status := "enabled"
	if !j.Enabled {
		status = "disabled"
	}
	line := fmt.Sprintf("%s [%s] (%s): %s", j.ID, status, j.Schedule, j.Instruction)
	if !j.Next.IsZero() {
		line += " — next " + j.Next.Format(time.RFC3339)
	}
	return line
}

func runGet(ctx context.Context, sched Scheduler, args Args) (tool.Result, error) {
	id := strings.TrimSpace(args.JobID)
	if id == "" {
		return tool.Result{}, fmt.Errorf("get requires job_id")
	}
	job, err := sched.Get(ctx, id)
	if err != nil {
		return tool.Result{}, fmt.Errorf("get job %s: %w", id, err)
	}
	return tool.Result{
		Content: formatJob(job),
		Data:    map[string]any{"job": job},
	}, nil
}

func runUpdate(ctx context.Context, sched Scheduler, args Args) (tool.Result, error) {
	id := strings.TrimSpace(args.JobID)
	if id == "" {
		return tool.Result{}, fmt.Errorf("update requires job_id")
	}
	schedule := strings.TrimSpace(args.Schedule)
	instruction := strings.TrimSpace(args.Instruction)
	if schedule == "" && instruction == "" {
		return tool.Result{}, fmt.Errorf("update requires a new schedule and/or instruction")
	}
	job, err := sched.Update(ctx, id, schedule, instruction)
	if err != nil {
		return tool.Result{}, fmt.Errorf("update job %s: %w", id, err)
	}
	return tool.Result{
		Content: "Updated job " + formatJob(job),
		Data:    map[string]any{"job": job},
	}, nil
}

func runSetEnabled(ctx context.Context, sched Scheduler, args Args, enabled bool) (tool.Result, error) {
	id := strings.TrimSpace(args.JobID)
	if id == "" {
		verb := "enable"
		if !enabled {
			verb = "disable"
		}
		return tool.Result{}, fmt.Errorf("%s requires job_id", verb)
	}
	job, err := sched.SetEnabled(ctx, id, enabled)
	if err != nil {
		return tool.Result{}, fmt.Errorf("set job %s enabled=%t: %w", id, enabled, err)
	}
	verb := "Enabled"
	if !enabled {
		verb = "Disabled"
	}
	return tool.Result{
		Content: fmt.Sprintf("%s job %s.", verb, job.ID),
		Data:    map[string]any{"job": job},
	}, nil
}

func runRemove(ctx context.Context, sched Scheduler, args Args) (tool.Result, error) {
	id := strings.TrimSpace(args.JobID)
	if id == "" {
		return tool.Result{}, fmt.Errorf("remove requires job_id")
	}
	if err := sched.Remove(ctx, id); err != nil {
		return tool.Result{}, fmt.Errorf("remove job %s: %w", id, err)
	}
	return tool.Result{
		Content: fmt.Sprintf("Removed job %s.", id),
		Data:    map[string]any{"removed": id},
	}, nil
}
