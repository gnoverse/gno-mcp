package indexer

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/budget"
	indexerpkg "github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterList wires the gno_list tool into s. The resolver maps a
// profile name to the indexerpkg.Client used to satisfy calls.
//
// gno_list browses the realm catalog by optional namespace/tag/category
// filters. It is NOT for reading realm content — use gno_render/gno_read
// for that. The listing echoes realm-supplied paths/descriptions, so it is
// returned wrapped in an untrusted-content envelope.
func RegisterList(s *server.Server, resolve indexerpkg.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_list",
		Description: "Filter-browse the on-chain realm catalog via the tx-indexer. " +
			"Returns a typed listing of deployed realms matching the given filters. " +
			"Use namespace, tag, or category to narrow the results. " +
			"NOT for reading realm content — use gno_render or gno_read for that.",
		InputSchema: listInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapIndexerRead,
		Annotations: server.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		Handler:     listHandler(resolve),
	})
}

func listHandler(resolve indexerpkg.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		namespace, err := server.StringArg(args, "namespace")
		if err != nil {
			return server.Result{}, err
		}
		tag, err := server.StringArg(args, "tag")
		if err != nil {
			return server.Result{}, err
		}
		category, err := server.StringArg(args, "category")
		if err != nil {
			return server.Result{}, err
		}
		profile, err := server.StringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("profile %q has no tx-indexer-url configured", profile)
		}

		realms, err := c.List(ctx, indexerpkg.ListFilter{
			Namespace: namespace,
			Tag:       tag,
			Category:  category,
		})
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_list: %w", err)
		}

		source := namespace
		if source == "" {
			source = "catalog"
		}
		text, _ := budget.Wrapped(formatRealms(realms), "", "realms", source)
		return server.Result{Text: text}, nil
	}
}

// formatRealms renders realms as a human-readable text listing.
func formatRealms(realms []indexerpkg.Realm) string {
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
