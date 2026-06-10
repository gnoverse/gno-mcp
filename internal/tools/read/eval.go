package read

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/budget"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterEval wires the gno_eval tool into s. The resolver maps a
// profile name to the chain.Client used to satisfy calls.
func RegisterEval(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_eval",
		Description: "Evaluates a Gno expression against a Gno package (realm or pure) and returns the typed " +
			"result as plain text. Use when the user or agent needs to inspect a value, call an exported " +
			"function, or read exported state without rendering content. Returns the vm/qeval typed result " +
			"(e.g. '(42 int)', '(\"hello\" string)') as OutputText, wrapped in an untrusted-content envelope " +
			"because the inner value is realm-controlled. " +
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
		path, err := server.StringArg(args, "path")
		if err != nil {
			return server.Result{}, err
		}
		if path == "" {
			return server.Result{}, fmt.Errorf("path is required (e.g. gno.land/r/myorg/counter)")
		}
		if !chain.IsReadablePackagePath(path) {
			return server.Result{}, fmt.Errorf("path must be a realm (gno.land/r/...) or pure package (gno.land/p/...); got %q", path)
		}
		expr, err := server.StringArg(args, "expr")
		if err != nil {
			return server.Result{}, err
		}
		if expr == "" {
			return server.Result{}, fmt.Errorf("expr is required (e.g. 'Counter()')")
		}
		profile, err := server.StringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}
		out, err := c.Eval(ctx, path, expr)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_eval: %w", err)
		}
		text, _ := budget.Wrapped(out, "", "eval", path)
		return server.Result{Text: text}, nil
	}
}

func evalInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"path": map[string]any{
			"type": "string",
			"description": "Package path: a realm (gno.land/r/myorg/counter) or pure package " +
				"(gno.land/p/myorg/lib). Required.",
			// Permissive hint only; authoritative validation is in the handler.
			"pattern": `^[a-z0-9][a-z0-9._/\-]+$`,
		},
		"expr": map[string]any{
			"type":        "string",
			"description": "Gno expression to evaluate within the package (e.g. 'Counter()' or 'Sprintf(\"%d\", 7)'). Required.",
		},
	}
	required := []string{"path", "expr"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
