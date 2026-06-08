// Package server wires the MCP server scaffolding, tool Registry, and profile-conditional schema.
package server

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// Capability tags a tool by class. It does NOT gate registration — the profile
// guards in main.go (AnyProfileHasIndexer, AnyProfileAgentCapable) decide which
// tools register. Its one functional use is audit logging: CapWrite and
// CapWritePrep tools are recorded to the audit log.
type Capability int

const (
	CapBaseRead    Capability = iota // read-only realm queries (render/read/eval/inspect/connect) + gno_key_address
	CapIndexerRead                   // tx-indexer reads; registered only when a profile sets tx-indexer-url
	CapWrite                         // chain writes: gno_call, gno_run, gno_addpkg, gno_key_generate (audited)
	CapWritePrep                     // session lifecycle: gno_auth_status, gno_session_propose, gno_session_revoke (audited)
	CapSessionRead                   // reserved; no tools use this capability
	CapA2A                           // reserved for a2a serve mode; no tools use this capability
)

// OutputKind says how the pipeline formats the handler's Result.
type OutputKind int

const (
	OutputText     OutputKind = iota // tool result text (default)
	OutputResource                   // MCP resource (untrusted realm content)
)

// Annotations carry MCP tool hints surfaced in the initialize/listTools response.
type Annotations struct {
	ReadOnly    bool
	Destructive bool
	Idempotent  bool
	OpenWorld   bool
}

// Validate returns an error when Annotations contain mutually exclusive flags.
// ReadOnly and Destructive cannot both be true: a tool that doesn't modify state
// cannot be destructive.
func (a Annotations) Validate() error {
	if a.ReadOnly && a.Destructive {
		return errors.New("annotations: ReadOnly and Destructive are mutually exclusive")
	}
	return nil
}

// Result is what a handler returns. Pipeline formats per OutputKind.
type Result struct {
	Text         string // for OutputText
	ResourceURI  string // for OutputResource
	ResourceBody string // for OutputResource
	ResourceMIME string // for OutputResource (defaults to text/markdown)
	// StructuredContent is arbitrary key-value data emitted alongside Text on the
	// MCP wire. Keys must be JSON-serializable. Nil and empty map are equivalent.
	StructuredContent map[string]any
	IsError           bool // true when the result represents a tool-side error
}

// ToolError is a structured error returned by a tool handler. Code/Message
// land in the MCP wire response; Extra fields merge into structuredContent.
type ToolError struct {
	Code    string
	Message string
	// Extra holds additional key-value pairs merged into the wire-format
	// structuredContent. Keys here take precedence over caller-provided keys
	// when both are present.
	Extra map[string]any
}

func (e *ToolError) Error() string {
	if len(e.Extra) == 0 {
		return fmt.Sprintf("tool error [%s]: %s", e.Code, e.Message)
	}
	keys := make([]string, 0, len(e.Extra))
	for k := range e.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("tool error [%s]: %s extra=%v", e.Code, e.Message, keys)
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
	Annotations Annotations
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

// Get returns the tool with the given name, ok=false if not registered.
func (r *Registry) Get(name string) (*Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Count() int { return len(r.tools) }

// WithCapability returns the tools matching c, sorted by Name for stable enumeration.
func (r *Registry) WithCapability(c Capability) []*Tool {
	var out []*Tool
	for _, t := range r.tools {
		if t.Capability == c {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// All returns every registered tool, sorted by Name for stable enumeration.
func (r *Registry) All() []*Tool {
	out := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Registry) Call(ctx context.Context, name string, args map[string]any) (Result, error) {
	t, ok := r.tools[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown tool: %s", name)
	}
	if t.Handler == nil {
		return Result{}, fmt.Errorf("tool %q has no handler", name)
	}
	return t.Handler(ctx, args)
}
