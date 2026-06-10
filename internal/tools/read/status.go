package read

import (
	"context"
	"fmt"
	"time"

	"github.com/gnoverse/gno-mcp/internal/budget"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterStatus wires the gno_status tool into s. The resolver maps a
// profile name to the chain.Client used to satisfy calls.
//
// gno_status answers "what chain am I talking to and is the node alive" for a
// profile: declared config (chain-id, RPC URL) plus the node's live tip. A
// node that reports a different chain-id than the profile declares is flagged
// as a mismatch — the same trust check gno_profile_add applies at add time.
func RegisterStatus(s *server.Server, resolve chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_status",
		Description: "Reports connection status for a chain profile: declared chain-id and RPC URL from config, " +
			"plus the node's live chain-id, latest block height, and block time. " +
			"Use to verify which chain a profile points at, whether the node is reachable, and how fresh its tip is. " +
			"Flags a mismatch when the node reports a different chain-id than the profile declares. " +
			"If the node is unreachable, config info is still returned with a height_error instead of failing. " +
			"Does NOT report peers, mempool, or validator health, and does NOT add profiles (use gno_connect + gno_profile_add). " +
			"Backed by the RPC /status endpoint.",
		InputSchema: statusInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapBaseRead,
		Annotations: server.Annotations{ReadOnly: true, Idempotent: true, OpenWorld: true},
		Handler:     statusHandler(s, resolve),
	})
}

func statusHandler(s *server.Server, resolve chain.Resolver) server.Handler {
	return func(ctx context.Context, args map[string]any) (server.Result, error) {
		profile, err := server.StringArg(args, "profile")
		if err != nil {
			return server.Result{}, err
		}
		p, ok := s.Config().Profiles[profile]
		if !ok {
			return server.Result{}, fmt.Errorf("unknown profile %q", profile)
		}

		structured := map[string]any{
			"profile":  profile,
			"chain_id": p.ChainID,
			"rpc_url":  p.RPCURL,
		}
		text := fmt.Sprintf("Profile %s: chain-id %s, rpc %s.", profile, p.ChainID, p.RPCURL)

		c := resolve(profile)
		if c == nil {
			return server.Result{}, fmt.Errorf("no chain client for profile %q", profile)
		}

		st, err := c.Status(ctx)
		if err != nil {
			// A dead node is a finding, not a tool failure: report config and
			// carry the query error per-field so the agent can still reason.
			structured["height_error"] = err.Error()
			text += fmt.Sprintf(" Live status query FAILED: %v.", err)
		} else {
			structured["node_chain_id"] = st.ChainID
			structured["height"] = st.Height
			text += fmt.Sprintf(" Node is up: latest block %d", st.Height)
			if !st.BlockTime.IsZero() {
				blockTime := st.BlockTime.UTC().Format(time.RFC3339)
				structured["block_time"] = blockTime
				text += " at " + blockTime
			}
			text += "."
			if st.ChainID != p.ChainID {
				text += fmt.Sprintf(" WARNING: chain-id mismatch — node reports %q but profile declares %q.", st.ChainID, p.ChainID)
			}
		}

		wrapped, _ := budget.Wrapped(text, "", "status", profile)
		return server.Result{Text: wrapped, StructuredContent: structured}, nil
	}
}

func statusInputSchema(s *server.Server) map[string]any {
	props := map[string]any{}
	var required []string
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
