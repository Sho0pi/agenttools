package search

import (
	"context"
	"os/exec"
	"strconv"
)

func DggProvdier(ctx context.Context, query string, max int) (string, error) {
	out, err := exec.CommandContext(ctx,
		"ddg-search",
		"--max-results", strconv.Itoa(max),
		query,
	).Output()
	return string(out), err
}
