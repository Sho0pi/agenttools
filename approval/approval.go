// Package approval provides the approval_request tool and the Approver provider
// it delegates to. The tool is a safety gate: before a risky or irreversible
// action, the agent requests explicit human sign-off. The Approver owns the
// approval channel and must fail closed — deny on timeout or error — so a
// missing approval never reads as a yes.
package approval

import (
	"context"
	"time"
)

// ApprovalRequest describes the action awaiting sign-off.
type ApprovalRequest struct {
	Summary           string         // human-readable description of the action
	RiskLevel         string         // "low" | "medium" | "high"
	AffectedResources []string       // what the action touches
	ProposedCommand   string         // optional exact command to be run
	ProposedPayload   map[string]any // optional structured payload
	Timeout           time.Duration  // 0 → Approver default
}

// Decision is the outcome of an approval request.
type Decision struct {
	Approved bool
	By       string // who decided (optional)
	Reason   string // optional rationale, especially on denial
}

// Approver routes an ApprovalRequest to a human (or policy) and returns the
// Decision. Implementations MUST fail closed: on timeout, cancellation, or
// channel error, return a non-approved Decision (or an error the tool treats as
// denial) rather than approving by default.
type Approver interface {
	RequestApproval(ctx context.Context, req ApprovalRequest) (Decision, error)
}

// ApproverFunc adapts a plain function to the Approver interface.
type ApproverFunc func(ctx context.Context, req ApprovalRequest) (Decision, error)

// RequestApproval implements Approver.
func (f ApproverFunc) RequestApproval(ctx context.Context, req ApprovalRequest) (Decision, error) {
	return f(ctx, req)
}

// DenyAll is an Approver that denies every request. It is the safe default for
// environments with no approval channel wired up: nothing risky proceeds.
type DenyAll struct{}

// RequestApproval implements Approver.
func (DenyAll) RequestApproval(_ context.Context, _ ApprovalRequest) (Decision, error) {
	return Decision{Approved: false, Reason: "no approval channel configured (deny by default)"}, nil
}
