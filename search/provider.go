package search

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DdgProvider runs the ddg-search CLI (github.com/Djarvur/ddg-search) and
// returns its stdout. It is the default Provider for the web_search tool; the
// ddg-search binary must be on PATH. On failure it includes any stderr output
// so the error is not opaque.
func DdgProvider(ctx context.Context, query string, max int) (string, error) {
	cmd := exec.CommandContext(ctx, "ddg-search", "--max-results", strconv.Itoa(max), query)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return string(out), nil
}
