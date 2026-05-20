package server

import (
	"context"
	"fmt"
)

// Capability classifies tools for conditional registration.
type Capability int

const (
	CapBaseRead    Capability = iota // always registered
	CapIndexerRead                   // only when any profile has tx-indexer-url
	CapWrite                         // Milestone B
	CapSessionRead                   // Milestone B
	CapWritePrep                     // Milestone B
	CapA2A                           // Milestone C
)

// OutputKind says how the pipeline formats the handler's Result.
type OutputKind int

const (
	OutputText     OutputKind = iota // tool result text (default)
	OutputResource                   // MCP resource (untrusted realm content)
)

// Result is what a handler returns. Pipeline formats per OutputKind.
type Result struct {
	Text         string // for OutputText
	ResourceURI  string // for OutputResource
	ResourceBody string // for OutputResource
	ResourceMIME string // for OutputResource (defaults to text/markdown)
}

// Handler is a tool's execution function. Pipeline injects schema-validated args.
type Handler func(ctx context.Context, args map[string]any) (Result, error)

// Tool is the declarative shape of an MCP tool registered with gnomcp.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any // JSON Schema fragment
	OutputKind  OutputKind
	Capability  Capability
	Handler     Handler
}

// Registry holds the declared tools and dispatches Call.
type Registry struct {
	tools map[string]*Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]*Tool{}}
}

func (r *Registry) Add(t *Tool) {
	r.tools[t.Name] = t
}

func (r *Registry) Count() int { return len(r.tools) }

func (r *Registry) WithCapability(c Capability) []*Tool {
	var out []*Tool
	for _, t := range r.tools {
		if t.Capability == c {
			out = append(out, t)
		}
	}
	return out
}

func (r *Registry) All() []*Tool {
	out := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (Result, error) {
	t, ok := r.tools[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown tool: %s", name)
	}
	return t.Handler(ctx, args)
}
