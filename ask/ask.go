// Package ask provides the ask_user tool and the Asker provider it delegates
// to. The tool lets an agent put a focused question to the human and wait for
// an answer. How the question reaches the user (chat reply, CLI prompt,
// webhook) and how the answer returns is the Asker's concern — the tool stays
// transport-neutral.
package ask

import "context"

// Question is a single request for user input.
type Question struct {
	Prompt  string   // the question to ask
	Context string   // optional background to help the user answer
	Options []string // optional suggested choices
	Default string   // value to assume if the user does not answer
	Urgency string   // "low" | "normal" | "high"
}

// Asker delivers a Question to the user and returns their answer. Implementations
// own the channel and should honour ctx cancellation / their own timeout, falling
// back to Question.Default rather than blocking forever.
type Asker interface {
	Ask(ctx context.Context, q Question) (answer string, err error)
}

// AskerFunc adapts a plain function to the Asker interface, so an integrator can
// wire up a chat or CLI prompt without declaring a type.
type AskerFunc func(ctx context.Context, q Question) (string, error)

// Ask implements Asker.
func (f AskerFunc) Ask(ctx context.Context, q Question) (string, error) { return f(ctx, q) }
