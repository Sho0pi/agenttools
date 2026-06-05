package ask

import (
	"context"
	"fmt"
	"strings"

	"github.com/sho0pi/agenttools/tool"
)

// Args are the ask_user arguments.
type Args struct {
	Question string   `json:"question"`
	Context  string   `json:"context"`
	Options  []string `json:"options"`
	Default  string   `json:"default"`
}

// New returns the ask_user tool. ask is the function that delivers questions to
// the user and returns their answer; it must not be nil.
func New(ask Ask) (tool.Tool, error) {
	if ask == nil {
		return nil, fmt.Errorf("ask: Ask must not be nil")
	}
	return tool.NewTypedTool(
		"ask_user",
		"Ask the user a single focused question when a decision is genuinely theirs and "+
			"you cannot resolve it from context, the task, or sensible defaults. Use sparingly: "+
			"do not ask about things you can infer or look up, do not ask permission for routine "+
			"steps, and batch related questions instead of interrupting repeatedly.",
		tool.Object(map[string]*tool.Property{
			"question": {Type: "string", Description: "The question to ask."},
			"context":  {Type: "string", Description: "Optional background to help the user answer."},
			"options":  {Type: "array", Description: "Optional suggested choices.", Items: &tool.Property{Type: "string"}},
			"default":  {Type: "string", Description: "Value to assume if the user does not answer."},
		}, "question"),
		func(ctx context.Context, args Args) (tool.Result, error) {
			return run(ctx, ask, args)
		},
	), nil
}

func run(ctx context.Context, ask Ask, args Args) (tool.Result, error) {
	if strings.TrimSpace(args.Question) == "" {
		return tool.Result{}, fmt.Errorf("question is required")
	}

	answer, err := ask(ctx, Question{
		Prompt:  args.Question,
		Context: args.Context,
		Options: args.Options,
		Default: args.Default,
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("ask user: %w", err)
	}
	return tool.Result{
		Content: answer,
		Data:    map[string]any{"question": args.Question, "answer": answer},
	}, nil
}
