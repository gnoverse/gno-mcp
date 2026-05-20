package read

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterEval wires the gno_eval tool into s. The resolver maps a
// profile name to the chain.Client used to satisfy calls.
func RegisterEval(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_eval",
		Description: "Evaluates a Gno expression against a realm and returns the typed result as plain text. " +
			"Use when the user or agent needs to inspect a value, call a pure function, or read exported " +
			"state without rendering realm content. Returns the vm/qeval typed result " +
			"(e.g. '(42 int)', '(\"hello\" string)') as OutputText — not an MCP resource, because the " +
			"value is gnomcp-typed metadata rather than untrusted realm content. " +
			"Does NOT return rendered markdown — use gno_render for that. " +
			"Backed by vm/qeval; HEAD-only (no historical heights).",
		InputSchema: evalInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapBaseRead,
		Handler:     evalHandler(resolve),
	})
}

func evalHandler(resolve chain.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		realm, err := stringArg(args, "realm")
		if err != nil {
			return server.Result{}, err
		}
		if realm == "" {
			return server.Result{}, fmt.Errorf("realm is required (e.g. gno.land/r/myorg/counter)")
		}
		expr, err := stringArg(args, "expr")
		if err != nil {
			return server.Result{}, err
		}
		if expr == "" {
			return server.Result{}, fmt.Errorf("expr is required (e.g. 'Counter()')")
		}
		profile, err := stringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}
		out, err := c.Eval(ctx, realm, expr)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_eval: %w", err)
		}
		return server.Result{Text: out}, nil
	}
}

func evalInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"realm": map[string]any{
			"type":        "string",
			"description": "Realm package path (e.g. 'gno.land/r/myorg/counter'). Required.",
			// Allow lowercase letters, digits, underscore, dot, hyphen, and slash.
			// Hyphen is needed for realms like gno.land/r/some-org/foo.
			"pattern": `^gno\.land/r/[a-z0-9_\-/\.]+$`,
		},
		"expr": map[string]any{
			"type":        "string",
			"description": "Gno expression to evaluate within the realm (e.g. 'Counter()' or 'GetBalance(\"addr\")'). Required.",
		},
	}
	required := []string{"realm", "expr"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
