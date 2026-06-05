// Package approval provides the approval_request tool. The Approve function
// type is the only provider seam: wire in any function that routes a request to
// a human (or policy) and returns the decision. Must fail closed — deny on
// timeout or error — so a missing approval never reads as a yes.
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
	Timeout           time.Duration  // 0 → provider default
}

// Decision is the outcome of an approval request.
type Decision struct {
	Approved bool
	By       string // who decided (optional)
	Reason   string // optional rationale, especially on denial
}

// Approve routes an ApprovalRequest to a human (or policy) and returns the
// Decision. Implementations must fail closed: on timeout, cancellation, or
// channel error, return a denied Decision or an error rather than approving.
type Approve func(ctx context.Context, req ApprovalRequest) (Decision, error)

// DenyAll is the safe default Approve for environments with no approval channel
// configured: every request is denied so nothing risky proceeds silently.
var DenyAll Approve = func(_ context.Context, _ ApprovalRequest) (Decision, error) {
	return Decision{Approved: false, Reason: "no approval channel configured (deny by default)"}, nil
}
