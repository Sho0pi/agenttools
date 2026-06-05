// Package ask provides the ask_user tool. The Ask function type is the only
// provider seam: wire in any function that delivers a Question to the user and
// returns their answer — no interface declaration required.
package ask

import "context"

// Question is a single request for user input.
type Question struct {
	Prompt  string   // the question to ask
	Context string   // optional background to help the user answer
	Options []string // optional suggested choices
	Default string   // value to assume if the user does not answer
}

// Ask delivers a Question to the user and returns their answer. Implementations
// own the channel and should honour ctx cancellation / their own timeout,
// falling back to Question.Default rather than blocking forever.
type Ask func(ctx context.Context, q Question) (answer string, err error)
