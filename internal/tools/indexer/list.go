package indexer

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterList wires the gno_list tool into s. The resolver maps a
// profile name to the indexer.Client used to satisfy calls.
//
// gno_list browses the realm catalog by optional namespace/tag/category
// filters. It is NOT for reading realm content — use gno_render/gno_read
// for that. Returns OutputText since listing is trusted server metadata.
func RegisterList(s *server.Server, resolve indexer.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_list",
		Description: "Filter-browse the on-chain realm catalog via the tx-indexer. " +
			"Returns a typed listing of deployed realms matching the given filters. " +
			"Use namespace, tag, or category to narrow the results. " +
			"NOT for reading realm content — use gno_render or gno_read for that. " +
			"Note: currently surfaces an 'error_unavailable' when the tx-indexer schema " +
			"does not expose the realms query; this is expected until the indexer is extended.",
		InputSchema: listInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapIndexerRead,
		Handler:     listHandler(resolve),
	})
}

func listHandler(resolve indexer.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		namespace, err := stringArg(args, "namespace")
		if err != nil {
			return server.Result{}, err
		}
		tag, err := stringArg(args, "tag")
		if err != nil {
			return server.Result{}, err
		}
		category, err := stringArg(args, "category")
		if err != nil {
			return server.Result{}, err
		}
		profile, err := stringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("profile %q has no tx-indexer-url configured", profile)
		}

		realms, err := c.List(ctx, indexer.ListFilter{
			Namespace: namespace,
			Tag:       tag,
			Category:  category,
		})
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_list: %w", err)
		}

		return server.Result{Text: formatRealms(realms)}, nil
	}
}

// formatRealms renders realms as a human-readable text listing.
func formatRealms(realms []indexer.Realm) string {
	if len(realms) == 0 {
		return "No realms matched the filter."
	}
	var sb strings.Builder
	for _, r := range realms {
		tags := strings.Join(r.Tags, ",")
		fmt.Fprintf(&sb, "- %s [%s] tags=%s\n  %s\n", r.Path, r.Category, tags, r.Description)
	}
	return sb.String()
}

func listInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"namespace": map[string]any{
			"type":        "string",
			"description": "Filter by namespace prefix (e.g. 'gno.land/r/demo'). Optional.",
		},
		"tag": map[string]any{
			"type":        "string",
			"description": "Filter by tag. Optional.",
		},
		"category": map[string]any{
			"type":        "string",
			"description": "Filter by category. Optional.",
		},
	}
	var required []string
	addProfileArg(s, props, &required)
	schema := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
