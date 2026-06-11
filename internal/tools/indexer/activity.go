package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/gnoverse/gno-mcp/internal/budget"
	indexerpkg "github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterActivity wires the gno_activity tool into s. The resolver maps a
// profile name to the indexerpkg.Client used to satisfy calls.
//
// gno_activity returns MsgCall and MsgRun transactions for a realm, optionally
// filtered to a closed [since, until] time range. Use gno_history for the full
// deploy+transaction log; use gno_read (outline) to explore function signatures.
func RegisterActivity(s *server.Server, resolve indexerpkg.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_activity",
		Description: "Return MsgCall and MsgRun activity for a realm via the tx-indexer. " +
			"Unlike gno_history (which returns every transaction including deploys), " +
			"gno_activity focuses on runtime activity and supports an optional time range " +
			"via RFC3339 since/until bounds. " +
			"Use gno_history for a full log including deploys; use gno_read (outline) to explore function signatures.",
		InputSchema: activityInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapIndexerRead,
		Annotations: server.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		Handler:     activityHandler(resolve),
	})
}

func activityHandler(resolve indexerpkg.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		realm, err := server.StringArg(args, "realm")
		if err != nil {
			return server.Result{}, err
		}
		if realm == "" {
			return server.Result{}, fmt.Errorf("realm: required argument is missing")
		}

		var since, until *time.Time

		if sv, err := server.StringArg(args, "since"); err != nil {
			return server.Result{}, err
		} else if sv != "" {
			t, err := time.Parse(time.RFC3339, sv)
			if err != nil {
				return server.Result{}, fmt.Errorf("invalid 'since' (must be RFC3339): %w", err)
			}
			since = &t
		}

		if uv, err := server.StringArg(args, "until"); err != nil {
			return server.Result{}, err
		} else if uv != "" {
			t, err := time.Parse(time.RFC3339, uv)
			if err != nil {
				return server.Result{}, fmt.Errorf("invalid 'until' (must be RFC3339): %w", err)
			}
			until = &t
		}

		profile, err := server.StringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("profile %q has no tx-indexer-url configured", profile)
		}

		events, err := c.Activity(ctx, realm, since, until)
		if err != nil {
			return server.Result{}, fmt.Errorf("gno_activity: %w", err)
		}

		text, _ := budget.Wrapped(formatEvents(events), "", "activity", realm)
		return server.Result{Text: text}, nil
	}
}

func activityInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"realm": map[string]any{
			"type":        "string",
			"description": "Realm path to fetch activity for (e.g. 'gno.land/r/demo/boards').",
			"pattern":     `^gno\.land/r/[a-z0-9_\-/\.]+$`,
		},
		"since": map[string]any{
			"type":        "string",
			"format":      "date-time",
			"description": "RFC3339 timestamp; only return events at or after this time (optional).",
		},
		"until": map[string]any{
			"type":        "string",
			"format":      "date-time",
			"description": "RFC3339 timestamp; only return events at or before this time (optional).",
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
