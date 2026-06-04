package approval

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sho0pi/agenttools/tool"
)

func TestNew_NilApprover(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil Approver")
	}
}

func TestApprovalRequest(t *testing.T) {
	var gotReq ApprovalRequest
	approver := ApproverFunc(func(_ context.Context, req ApprovalRequest) (Decision, error) {
		gotReq = req
		return Decision{Approved: true, By: "alice"}, nil
	})
	tr, err := New(approver)
	if err != nil {
		t.Fatal(err)
	}
	do := func(args Args) (tool.Result, error) {
		raw, _ := json.Marshal(args)
		return tr.Execute(context.Background(), raw)
	}

	res, err := do(Args{ActionSummary: "force-push main", RiskLevel: "high", ProposedCommand: "git push --force"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["approved"] != true || !strings.Contains(res.Content, "APPROVED") {
		t.Fatalf("got %+v", res)
	}
	if gotReq.ProposedCommand != "git push --force" {
		t.Fatalf("request not passed through: %+v", gotReq)
	}
}

func TestApprovalRequest_Denied(t *testing.T) {
	tr, _ := New(DenyAll{})
	raw, _ := json.Marshal(Args{ActionSummary: "delete prod", RiskLevel: "high"})
	res, err := tr.Execute(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if res.Data["approved"] != false || !strings.Contains(res.Content, "DENIED") {
		t.Fatalf("DenyAll should deny: %+v", res)
	}
}

func TestApprovalRequest_FailsClosedOnError(t *testing.T) {
	tr, _ := New(ApproverFunc(func(_ context.Context, _ ApprovalRequest) (Decision, error) {
		return Decision{}, errors.New("channel down")
	}))
	raw, _ := json.Marshal(Args{ActionSummary: "x", RiskLevel: "medium"})
	if _, err := tr.Execute(context.Background(), raw); err == nil {
		t.Fatal("approver error must surface as an error (fail closed), not silent approval")
	}
}

func TestApprovalRequest_Validation(t *testing.T) {
	tr, _ := New(DenyAll{})
	do := func(args Args) error {
		raw, _ := json.Marshal(args)
		_, err := tr.Execute(context.Background(), raw)
		return err
	}
	if do(Args{RiskLevel: "high"}) == nil {
		t.Fatal("missing action_summary should error")
	}
	if do(Args{ActionSummary: "x"}) == nil {
		t.Fatal("missing/invalid risk_level should error")
	}
	if do(Args{ActionSummary: "x", RiskLevel: "extreme"}) == nil {
		t.Fatal("invalid risk_level should error")
	}
}
