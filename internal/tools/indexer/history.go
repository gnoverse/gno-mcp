package indexer

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/budget"
	"github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterHistory wires the gno_history tool into s. The resolver maps a
// profile name to the indexer.Client used to satisfy calls.
//
// gno_history returns the full deploy + transaction log for a realm in
// chronological order. Use gno_activity to filter by time range, and
// gno_inspect to explore function signatures.
func RegisterHistory(s *server.Server, resolve indexer.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_history",
		Description: "Return the complete deploy and transaction log for a realm via the tx-indexer. " +
			"Lists every transaction touching the realm in chronological order, including deploys " +
			"(MsgAddPackage), calls (MsgCall), and script runs (MsgRun). " +
			"Use gno_activity to filter by time range; use gno_inspect to explore function signatures.",
		InputSchema: historyInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapIndexerRead,
		Annotations: server.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		Handler:     historyHandler(resolve),
	})
}

func historyHandler(resolve indexer.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		realm, err := stringArg(args, "realm")
		if err != nil {
			return server.Result{}, err
		}
		if realm == "" {
			return server.Result{}, fmt.Errorf("realm: required argument is missing")
		}

		profile, err := stringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("profile %q has no tx-indexer-url configured", profile)
		}

		events, err := c.History(ctx, realm)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_history: %w", err)
		}

		r := budget.Apply(formatEvents(events), "", false)
		text := r.Full
		if r.Truncated {
			text = r.Summary
		}
		return server.Result{Text: text}, nil
	}
}

// formatEvents renders TxEvents as a markdown-friendly bulleted listing.
// Used by both gno_history and gno_activity.
func formatEvents(events []indexer.TxEvent) string {
	if len(events) == 0 {
		return "No transactions found for this realm."
	}
	var b strings.Builder
	for _, e := range events {
		fmt.Fprintf(&b, "- %s @ height %d (%s)  kind=%s",
			e.Time.UTC().Format("2006-01-02 15:04:05"), e.Height, e.Hash, e.Kind)
		if e.Caller != "" {
			b.WriteString("  caller=")
			b.WriteString(e.Caller)
		}
		if e.Func != "" {
			b.WriteString("  func=")
			b.WriteString(e.Func)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func historyInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"realm": map[string]any{
			"type":        "string",
			"description": "Realm path to fetch history for (e.g. 'gno.land/r/demo/boards').",
			"pattern":     `^gno\.land/r/[a-z0-9_\-/\.]+$`,
		},
	}
	required := []string{"realm"}
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
