# agenttools

[![CI](https://github.com/Sho0pi/agenttools/actions/workflows/ci.yml/badge.svg)](https://github.com/Sho0pi/agenttools/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/sho0pi/agenttools.svg)](https://pkg.go.dev/github.com/sho0pi/agenttools)
[![Go Report Card](https://goreportcard.com/badge/github.com/sho0pi/agenttools)](https://goreportcard.com/report/github.com/sho0pi/agenttools)

A provider-neutral **tool ecosystem for AI agents written in Go**.

A tool exposes a name, description, and JSON schema to the model; the model
returns a tool call; the agent dispatches it by name and feeds the result back.
Nothing in a tool knows which LLM provider is in use — provider adapters
translate the neutral schema into each vendor's wire format.

## Install

```sh
go get github.com/sho0pi/agenttools@latest
```

Requires **Go 1.24+**.

## Quick start

```go
package main

import (
	"context"
	"fmt"

	"github.com/sho0pi/agenttools/search"
	"github.com/sho0pi/agenttools/tool"
)

func main() {
	// Build a registry and register the tools you want to expose.
	reg := tool.NewRegistry()

	webSearch, err := search.New(search.DdgProvider) // requires the ddg-search CLI on PATH
	if err != nil {
		panic(err)
	}
	reg.Register(webSearch)

	// Hand reg.Tools() to your provider adapter so the model sees the schemas.
	for _, t := range reg.Tools() {
		fmt.Printf("%s: %s\n", t.Name(), t.Description())
	}

	// When the model emits a tool call, dispatch it by name. args is the
	// decoded argument map your provider SDK hands back.
	res, err := reg.Dispatch(context.Background(), "web_search", map[string]any{
		"query": "best go web frameworks 2025",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(res.Content) // feed this back to the model
}
```

A complete, runnable wiring example lives in [`examples/basic`](examples/basic).

## Core concepts

The `tool` package is the foundation:

| Type | Purpose |
|------|---------|
| `tool.Tool` | The interface every tool implements: `Name`, `Description`, `Schema`, `Execute`. |
| `tool.TypedTool[T]` | Adapts a typed handler `func(ctx, T) (Result, error)` to `Tool`, removing JSON-unmarshal boilerplate. |
| `tool.Schema` / `tool.Property` | JSON Schema for tool arguments; serializes directly to what OpenAI/Claude APIs expect. |
| `tool.Result` | What a tool returns: `Content` (text for the model) + optional structured `Data`. |
| `tool.Registry` | Concurrency-safe registration and `Dispatch` by name. |

Every tool constructor returns `(tool.Tool, error)` and **never panics** — this
is a library, so validation failures are returned to the caller.

### The provider seam

Tools never touch the outside world directly. Each tool takes an injected
**provider** (a small interface or function) that owns I/O, time, or sandboxing.
This keeps tools testable without network/clock access and lets you swap
implementations:

- `search` takes a `Provider` func (default: `search.DdgProvider`).
- `webfetch` takes a `func() Config` and an optional `Summarizer`.
- `cron` takes a `Scheduler` interface (back it with gocron, a DB, etc.).

## Bundled tools

| Package | Tool name | What it does |
|---------|-----------|--------------|
| `search` | `web_search` | DuckDuckGo search via the `ddg-search` CLI. Returns titles, URLs, snippets. |
| `webfetch` | `web_fetch` | Fetches up to 5 URLs concurrently, converts HTML→markdown, optionally LLM-summarizes large pages. SSRF-guarded. |
| `cron` | `cron` | Schedule/list/get/update/remove/enable/disable recurring instructions via an injected `Scheduler`. |

More tools are tracked as issues — see the repo's issue list.

## Layout

```
agenttools/
├── tool/          # Core: Tool interface, TypedTool, Schema, Result, Registry
├── search/        # web_search tool
├── webfetch/      # web_fetch tool (+ HTML→markdown, SSRF guard)
├── cron/          # cron tool
└── examples/
    └── basic/     # runnable wiring example
```

## Development

```sh
make test       # go test -race ./...
make lint       # golangci-lint run
make fmt        # gofmt -w .
make ci         # everything CI runs
```

CI runs `go vet`, `go test -race`, `golangci-lint`, and `go build` on every push
and PR (see [`.github/workflows/ci.yml`](.github/workflows/ci.yml)).

## Releasing

This is an import-only Go module — releases are git tags, no binaries.

```sh
git tag v0.1.0
git push origin v0.1.0
```

Pushing a `vX.Y.Z` tag triggers
[`.github/workflows/release.yml`](.github/workflows/release.yml), which
re-runs the test suite and publishes a GitHub release. Consumers then pin a
version with `go get github.com/sho0pi/agenttools@v0.1.0`.

## License

[MIT](LICENSE) © Itay Blokh
