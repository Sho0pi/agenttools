package approval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sho0pi/agenttools/tool"
)

// Args are the approval_request arguments.
type Args struct {
	ActionSummary     string         `json:"action_summary"`
	RiskLevel         string         `json:"risk_level"`
	AffectedResources []string       `json:"affected_resources"`
	ProposedCommand   string         `json:"proposed_command"`
	ProposedPayload   map[string]any `json:"proposed_payload"`
	TimeoutSec        int            `json:"timeout_sec"`
}

// New returns the approval_request tool backed by approver, or an error if
// approver is nil.
func New(approver Approver) (tool.Tool, error) {
	if approver == nil {
		return nil, fmt.Errorf("approval: Approver must not be nil")
	}
	return tool.NewTypedTool(
		"approval_request",
		"Request human approval before any high-impact or irreversible action (deleting "+
			"data, spending money, sending external messages, changing accounts, force-pushing). "+
			"REQUIRED for such actions — do not proceed unless the returned decision is approved. "+
			"Do not use it for routine, reversible, read-only steps; that only creates approval fatigue.",
		tool.Object(map[string]*tool.Property{
			"action_summary":     {Type: "string", Description: "What the action does, in plain language."},
			"risk_level":         {Type: "string", Description: "Risk of the action.", Enum: []string{"low", "medium", "high"}},
			"affected_resources": {Type: "array", Description: "Resources the action touches.", Items: &tool.Property{Type: "string"}},
			"proposed_command":   {Type: "string", Description: "Exact command to be run, if any."},
			"proposed_payload":   {Type: "object", Description: "Structured payload of the action, if any."},
			"timeout_sec":        {Type: "integer", Description: "How long to wait for a decision (Approver default if unset)."},
		}, "action_summary", "risk_level"),
		func(ctx context.Context, args Args) (tool.Result, error) {
			return run(ctx, approver, args)
		},
	), nil
}

func run(ctx context.Context, approver Approver, args Args) (tool.Result, error) {
	if strings.TrimSpace(args.ActionSummary) == "" {
		return tool.Result{}, fmt.Errorf("action_summary is required")
	}
	switch args.RiskLevel {
	case "low", "medium", "high":
	default:
		return tool.Result{}, fmt.Errorf("risk_level must be one of low, medium, high (got %q)", args.RiskLevel)
	}

	decision, err := approver.RequestApproval(ctx, ApprovalRequest{
		Summary:           args.ActionSummary,
		RiskLevel:         args.RiskLevel,
		AffectedResources: args.AffectedResources,
		ProposedCommand:   args.ProposedCommand,
		ProposedPayload:   args.ProposedPayload,
		Timeout:           time.Duration(args.TimeoutSec) * time.Second,
	})
	if err != nil {
		// Fail closed: an approval error is a denial, surfaced as an error so the
		// agent does not proceed.
		return tool.Result{}, fmt.Errorf("approval failed (treat as denied): %w", err)
	}

	verb := "DENIED"
	if decision.Approved {
		verb = "APPROVED"
	}
	content := fmt.Sprintf("%s: %s", verb, args.ActionSummary)
	if decision.Reason != "" {
		content += " — " + decision.Reason
	}
	return tool.Result{
		Content: content,
		Data: map[string]any{
			"approved": decision.Approved,
			"by":       decision.By,
			"reason":   decision.Reason,
		},
	}, nil
}
