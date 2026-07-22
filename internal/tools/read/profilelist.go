package read

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterProfileList wires the gno_profile_list tool into s: the catalog of
// loaded profiles, mapping profile names to chain-ids, endpoints, and
// lifecycle status. Config-derived only — it never dials a node, so the
// output carries no untrusted chain content.
func RegisterProfileList(s *server.Server) {
	s.Registry().Add(&server.Tool{
		Name: "gno_profile_list",
		Description: "Lists every loaded chain profile with its chain-id, endpoints, and lifecycle status — " +
			"the map between profile names and the chains they reach (e.g. profile 'testnet' -> chain topaz-1). " +
			"Use when the user names a chain or network ('on topaz', 'on test13') to resolve which profile to pass " +
			"to the other tools, or to see which chains are configured at all. " +
			"Returns one entry per profile: name, chain-id, kind (local | testnet | read-only), a sunset flag " +
			"(a retiring testnet — still fully writable, but prefer the current testnet for new work), " +
			"and the configured endpoints (RPC, gnoweb, tx-indexer, agent-faucet). " +
			"Mainnet/betanet profiles are read-only: no agent key, faucet, or session. " +
			"Does NOT dial any node — for a live height/chain-id check use gno_status; " +
			"to reach a chain not listed here use gno_connect and gno_profile_add. Takes no arguments.",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false},
		OutputKind:  server.OutputText,
		Capability:  server.CapBaseRead,
		Annotations: server.Annotations{ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false},
		Handler: func(_ context.Context, _ map[string]any) (server.Result, error) {
			return profileListHandler(s), nil
		},
	})
}

// profileStatus is the one-line lifecycle label shown per profile.
func profileStatus(p profiles.Profile) string {
	switch {
	case p.Sunset:
		return "sunset — retiring chain, still writable; prefer the current testnet for new work"
	case p.IsLocal():
		return "local dev chain"
	case p.IsTestnet():
		return "current testnet, writable"
	default:
		return "read-only (mainnet/betanet)"
	}
}

func profileListHandler(s *server.Server) server.Result {
	cfg := s.Config()
	names := make([]string, 0, len(cfg.Profiles))
	for n := range cfg.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	list := make([]map[string]any, 0, len(names))
	fmt.Fprintf(&b, "%d profiles loaded:\n", len(names))
	for _, name := range names {
		p := cfg.Profiles[name]
		fmt.Fprintf(&b, "\n- %s — chain %s · %s\n", name, p.ChainID, profileStatus(p))
		fmt.Fprintf(&b, "  rpc %s", p.RPCURL)
		for _, ep := range []struct{ label, url string }{
			{"gnoweb", p.GnowebURL},
			{"indexer", p.TxIndexerURL},
			{"faucet-service", p.FaucetServiceURL},
			{"faucet-page", p.FaucetURL},
		} {
			if ep.url != "" {
				fmt.Fprintf(&b, " · %s %s", ep.label, ep.url)
			}
		}
		b.WriteString("\n")

		list = append(list, map[string]any{
			"name":               name,
			"chain_id":           p.ChainID,
			"kind":               p.Kind(),
			"sunset":             p.Sunset,
			"rpc_url":            p.RPCURL,
			"gnoweb_url":         p.GnowebURL,
			"tx_indexer_url":     p.TxIndexerURL,
			"faucet_service_url": p.FaucetServiceURL,
			"faucet_url":         p.FaucetURL,
		})
	}
	return server.Result{
		Text:              b.String(),
		StructuredContent: map[string]any{"profiles": list},
	}
}
