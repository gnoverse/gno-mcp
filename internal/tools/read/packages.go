package read

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/budget"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterPackages wires the gno_packages tool into s. The resolver maps a
// profile name to the chain.Client used to satisfy calls.
//
// gno_packages discovers package paths under a prefix or namespace via
// vm/qpaths — chain-native, no tx-indexer required. It complements gno_list
// (indexer catalog search) and gno_read (source).
func RegisterPackages(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_packages",
		Description: "Lists package paths deployed under a path on-chain. " +
			"Use to discover which realms/pure packages exist under a namespace before reading them. " +
			"Accepts a path prefix ('gno.land/r/demo/' returns everything under it, both /r/ and /p/) " +
			"or '@namespace' ('@demo' returns gno.land/p/demo/* and gno.land/r/demo/*). " +
			"Returns a newline-separated list of fully-qualified package paths (no metadata). " +
			"Does NOT read source (use gno_read) or search by tag/category (use gno_list, which needs a tx-indexer). " +
			"Backed by vm/qpaths; HEAD-only.",
		InputSchema: packagesInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapBaseRead,
		Annotations: server.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		Handler:     packagesHandler(resolve),
	})
}

func packagesHandler(resolve chain.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		path, err := stringArg(args, "path")
		if err != nil {
			return server.Result{}, err
		}
		if path == "" {
			return server.Result{}, fmt.Errorf("path is required (e.g. 'gno.land/r/demo/' or '@demo')")
		}
		profile, err := stringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		limit := 0
		if raw, ok := args["limit"]; ok {
			n, ok := raw.(float64)
			if !ok || n != float64(int(n)) {
				return server.Result{}, fmt.Errorf("limit: expected an integer, got %v", raw)
			}
			if n < 0 {
				return server.Result{}, fmt.Errorf("limit: must be >= 0, got %d", int(n))
			}
			limit = int(n)
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}

		paths, err := c.ListPaths(ctx, path, limit)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_packages: %w", err)
		}

		r := budget.Apply(strings.Join(paths, "\n"), "", false)
		text := r.Full
		if r.Truncated {
			text = r.Summary
		}
		return server.Result{Text: text}, nil
	}
}

func packagesInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Path prefix ('gno.land/r/demo/') or '@namespace' ('@demo' for both /p/ and /r/). Required.",
		},
		"limit": map[string]any{
			"type":        "integer",
			"description": "Max paths to return. Default 1000; the chain caps at 10000.",
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
