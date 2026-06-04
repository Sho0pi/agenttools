package ask

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sho0pi/agenttools/tool"
)

func TestNew_NilAsker(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected error for nil Asker")
	}
}

func TestAskUser(t *testing.T) {
	var gotQ Question
	asker := AskerFunc(func(_ context.Context, q Question) (string, error) {
		gotQ = q
		return "Postgres", nil
	})
	tr, err := New(asker)
	if err != nil {
		t.Fatal(err)
	}
	do := func(args Args) (tool.Result, error) {
		raw, _ := json.Marshal(args)
		return tr.Execute(context.Background(), raw)
	}

	res, err := do(Args{Question: "Which DB?", Options: []string{"Postgres", "MySQL"}, Default: "Postgres"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "Postgres" || res.Data["answer"] != "Postgres" {
		t.Fatalf("got %+v", res)
	}
	if gotQ.Prompt != "Which DB?" || len(gotQ.Options) != 2 {
		t.Fatalf("question not passed through: %+v", gotQ)
	}

	// validation
	if _, err := do(Args{Question: "  "}); err == nil {
		t.Fatal("blank question should error")
	}
	if _, err := do(Args{Question: "x", Urgency: "bogus"}); err == nil {
		t.Fatal("invalid urgency should error")
	}
}

func TestAskUser_AskerError(t *testing.T) {
	tr, _ := New(AskerFunc(func(_ context.Context, _ Question) (string, error) {
		return "", errors.New("user unreachable")
	}))
	raw, _ := json.Marshal(Args{Question: "x?"})
	if _, err := tr.Execute(context.Background(), raw); err == nil {
		t.Fatal("asker error should propagate")
	}
}
