// Command basic shows how to wire the agenttools registry: register a custom
// tool plus a bundled one, list their schemas (what you hand to an LLM provider
// adapter), and dispatch a tool call by name.
//
// Run it with:
//
//	go run ./examples/basic
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sho0pi/agenttools/search"
	"github.com/sho0pi/agenttools/tool"
)

// greetArgs is the typed argument struct for the custom "greet" tool. Its json
// tags match the Schema below.
type greetArgs struct {
	Name string `json:"name"`
}

func main() {
	reg := tool.NewRegistry()

	// A custom tool built from a typed handler — no manual JSON decoding.
	greet := tool.NewTypedTool(
		"greet",
		"Greet a person by name.",
		tool.Object(map[string]*tool.Property{
			"name": {Type: "string", Description: "Who to greet."},
		}, "name"),
		func(_ context.Context, args greetArgs) (tool.Result, error) {
			return tool.Result{Content: "Hello, " + args.Name + "!"}, nil
		},
	)
	reg.Register(greet)

	// A bundled tool. search.DdgProvider needs the ddg-search CLI on PATH; we
	// register it to show the wiring but only dispatch the offline tool below.
	webSearch, err := search.New(search.DdgProvider)
	if err != nil {
		log.Fatalf("build web_search: %v", err)
	}
	reg.Register(webSearch)

	// These are the schemas you would translate into your LLM provider's
	// tool/function-calling format.
	fmt.Println("registered tools:")
	for _, t := range reg.Tools() {
		fmt.Printf("  - %s: %s\n", t.Name(), t.Description())
	}

	// Simulate the model calling "greet". args is the decoded argument map a
	// provider SDK hands back after the model emits a tool call.
	res, err := reg.Dispatch(context.Background(), "greet", map[string]any{"name": "Ada"})
	if err != nil {
		log.Fatalf("dispatch: %v", err)
	}
	fmt.Println("\ntool result:", res.Content)
}
