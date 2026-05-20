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
		Description: "Returns the godoc summary of a Gno realm: package description, exported types, " +
			"function signatures, and per-symbol docstrings as plain text. " +
			"Use when the user or agent needs to understand the API surface of a realm without reading " +
			"the full source. Returns OutputText (typed metadata, not realm content). " +
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
		realm, err := stringArg(args, "realm")
		if err != nil {
			return server.Result{}, err
		}
		if realm == "" {
			return server.Result{}, fmt.Errorf("realm is required (e.g. gno.land/r/myorg/foo)")
		}
		profile, err := stringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}
		doc, err := c.Doc(ctx, realm)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_inspect: %w", err)
		}
		return server.Result{Text: doc}, nil
	}
}

func inspectInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"realm": map[string]any{
			"type":        "string",
			"description": "Realm package path (e.g. 'gno.land/r/myorg/foo'). Required.",
			// Allow lowercase letters, digits, underscore, dot, hyphen, and slash.
			// Hyphen is needed for realms like gno.land/r/some-org/foo.
			"pattern": `^gno\.land/r/[a-z0-9_\-/\.]+$`,
		},
	}
	required := []string{"realm"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
