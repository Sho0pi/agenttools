// Package askcli provides a synchronous CLI provider for the ask_user tool.
// Questions are printed to an io.Writer; answers are read from an io.Reader.
// Wire it up with os.Stdin / os.Stderr for an interactive terminal agent.
package askcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/sho0pi/agenttools/ask"
)

// New returns an ask.Ask that prints questions to out and reads answers from in.
// Context cancellation interrupts the read and returns the question's default.
func New(in io.Reader, out io.Writer) ask.Ask {
	scanner := bufio.NewScanner(in)
	return func(ctx context.Context, q ask.Question) (string, error) {
		print(out, q)

		done := make(chan string, 1)
		go func() {
			if scanner.Scan() {
				done <- scanner.Text()
			} else {
				done <- ""
			}
		}()

		select {
		case <-ctx.Done():
			return q.Default, ctx.Err()
		case text := <-done:
			text = strings.TrimSpace(text)
			if text == "" {
				return q.Default, nil
			}
			return text, nil
		}
	}
}

func print(out io.Writer, q ask.Question) {
	_, _ = fmt.Fprintln(out, q.Prompt)
	if q.Context != "" {
		_, _ = fmt.Fprintln(out, "  ("+q.Context+")")
	}
	for i, o := range q.Options {
		_, _ = fmt.Fprintf(out, "  %d) %s\n", i+1, o)
	}
	if q.Default != "" {
		_, _ = fmt.Fprintf(out, "[default: %s] ", q.Default)
	}
}
