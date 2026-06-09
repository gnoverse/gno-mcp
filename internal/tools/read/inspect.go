package read

import (
	"context"
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterInspect wires the gno_inspect tool into s. The resolver maps a
// profile name to the chain.Client used to satisfy calls.
func RegisterInspect(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_inspect",
		Description: "Returns the godoc summary of a Gno package (realm or pure): package description, " +
			"exported types, function signatures, and per-symbol docstrings as plain text. " +
			"Use when the user or agent needs to understand the API surface of a package without reading " +
			"the full source. Returns OutputText (typed metadata, not package content). " +
			"For the raw source use gno_read; for rendered output use gno_render; for value inspection " +
			"use gno_eval. Backed by vm/qdoc; HEAD-only.",
		InputSchema: inspectInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapBaseRead,
		Handler:     inspectHandler(resolve),
	})
}

func inspectHandler(resolve chain.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		path, err := stringArg(args, "path")
		if err != nil {
			return server.Result{}, err
		}
		if path == "" {
			return server.Result{}, fmt.Errorf("path is required (e.g. gno.land/r/myorg/foo)")
		}
		if !chain.IsReadablePackagePath(path) {
			return server.Result{}, fmt.Errorf(
				"path must be a realm (gno.land/r/...) or pure package (gno.land/p/...); got %q", path)
		}
		profile, err := stringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}
		doc, err := c.Doc(ctx, path)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_inspect: %w", err)
		}
		text, _ := budgetBody(doc, "")
		return server.Result{Text: text}, nil
	}
}

func inspectInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"path": map[string]any{
			"type": "string",
			"description": "Package path: a realm (gno.land/r/myorg/foo) or pure package " +
				"(gno.land/p/myorg/lib). Required.",
			// Permissive hint only; authoritative validation is in the handler.
			"pattern": `^[a-z0-9][a-z0-9._/\-]+$`,
		},
	}
	required := []string{"path"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
