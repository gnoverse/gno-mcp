// Package admin holds server-administration tools: tools that change gnomcp's
// own runtime state rather than reading or writing a chain.
package admin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gnoverse/gno-mcp/internal/gnoweb"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// verifyTimeout bounds the live chain-id check against the candidate RPC.
const verifyTimeout = 10 * time.Second

// ChainIDVerifier reports the chain-id a node at rpcURL declares.
// Production wiring uses chain.QueryChainID; tests substitute fakes.
type ChainIDVerifier func(ctx context.Context, rpcURL string) (string, error)

// RegisterProfileAdd wires the gno_profile_add tool into s. gnowebClient
// fetches gnoweb pages for the discovery form; verify performs the live
// chain-id cross-check; onAdded re-registers and re-publishes the tool set
// after a successful add (the caller serializes concurrent republishes).
func RegisterProfileAdd(s *server.Server, gnowebClient *http.Client, verify ChainIDVerifier, onAdded func() error) {
	s.Registry().Add(&server.Tool{
		Name: "gno_profile_add",
		Description: "Adds a chain profile to this gnomcp process, in-memory only — it lasts until restart " +
			"and never touches profiles.toml (the result includes the CLI command to persist it). " +
			"Use when the user or agent wants to read or write a gno chain that is not in the current " +
			"profile list, e.g. after gno_connect discovers one. Two input forms (exactly one): " +
			"rpc_url + chain_id (explicit), or gnoweb_url (discovers them from the page's gnoconnect " +
			"meta-tags; treated as a hint — the node is dialed and must report the same chain-id either way). " +
			"dev and numbered testnets are write-capable; any other chain (mainnet/betanet, e.g. gnoland1) is " +
			"admitted READ-ONLY — readable via the read tools, but with no agent key, faucet, or write path, " +
			"which is exactly what auditing deployed source on gno.land needs. " +
			"Profiles loaded at startup cannot be overridden; re-adding a profile created by this tool replaces it. " +
			"Dynamic profiles carry no master-address: writable ones support reads and agent-key writes only — " +
			"write-as-user sessions need a profile the user persisted in profiles.toml. " +
			"Note: an agent key generated for a dynamic testnet profile persists on disk across restarts " +
			"even though the profile does not; re-adding the profile reattaches the key.",
		InputSchema: profileAddInputSchema(),
		OutputKind:  server.OutputText,
		Capability:  server.CapWritePrep,
		// Idempotent: a same-args re-add replaces the entry with an identical
		// profile and converges, so retrying after republish_failed is safe.
		Annotations: server.Annotations{ReadOnly: false, Destructive: false, Idempotent: true, OpenWorld: true},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return profileAddHandler(ctx, args, s, gnowebClient, verify, onAdded)
		},
	})
}

func profileAddInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type": "string",
				"description": "Profile name: lowercase alphanumeric, internal '-' or '_' allowed (e.g. 'test13'). " +
					"Must not be 'default' or a name already loaded from profiles.toml/builtins. Required.",
				"pattern": `^[a-z0-9][a-z0-9_-]*$`,
			},
			"rpc_url": map[string]any{
				"type":   "string",
				"format": "uri",
				"description": "Chain RPC endpoint (absolute http(s) URL, e.g. 'https://rpc.test13.gno.land'). " +
					"Required together with chain_id unless gnoweb_url is given.",
			},
			"chain_id": map[string]any{
				"type": "string",
				"description": "Chain-id the node reports (e.g. 'test-13', or 'gnoland1' for betanet — read-only). " +
					"dev/testNN are write-capable; any other id is admitted read-only. " +
					"Required together with rpc_url unless gnoweb_url is given. Cross-checked against the live node.",
			},
			"gnoweb_url": map[string]any{
				"type":   "string",
				"format": "uri",
				"description": "Gnoweb page URL to discover rpc_url + chain_id from (e.g. 'https://test13.testnets.gno.land'). " +
					"Mutually exclusive with rpc_url/chain_id. Discovery is a hint only — the node is still dialed and verified.",
			},
			"tx_indexer_url": map[string]any{
				"type":   "string",
				"format": "uri",
				"description": "Optional tx-indexer GraphQL endpoint (e.g. 'https://indexer.test13.testnets.gno.land/graphql/query'); " +
					"enables the indexer tools for this profile.",
			},
			"faucet_service_url": map[string]any{
				"type":        "string",
				"format":      "uri",
				"description": "Optional automatic agent-faucet service URL (e.g. 'http://127.0.0.1:8590'; testnet funding tier 2).",
			},
			"faucet_url": map[string]any{
				"type":        "string",
				"format":      "uri",
				"description": "Optional human faucet page URL (e.g. 'https://faucet.gno.land'; testnet funding tier 1).",
			},
		},
		"required":             []string{"name"},
		"additionalProperties": false,
	}
}

func profileAddHandler(ctx context.Context, args map[string]any, s *server.Server, gnowebClient *http.Client, verify ChainIDVerifier, onAdded func() error) (server.Result, error) {
	var name, rpcURL, chainID, gnowebURL string
	p := profiles.Profile{}
	stringArgs := map[string]*string{
		"name": &name, "rpc_url": &rpcURL, "chain_id": &chainID, "gnoweb_url": &gnowebURL,
		"tx_indexer_url": &p.TxIndexerURL, "faucet_service_url": &p.FaucetServiceURL, "faucet_url": &p.FaucetURL,
	}
	for arg, dst := range stringArgs {
		v, err := server.StringArg(args, arg)
		if err != nil {
			return server.Result{}, err
		}
		*dst = v
	}

	explicit := rpcURL != "" || chainID != ""
	switch {
	case gnowebURL != "" && explicit:
		return server.Result{}, &server.ToolError{
			Code:    "invalid_arguments",
			Message: "pass either gnoweb_url OR rpc_url+chain_id, not both",
		}
	case gnowebURL == "" && (rpcURL == "" || chainID == ""):
		return server.Result{}, &server.ToolError{
			Code:    "invalid_arguments",
			Message: "rpc_url and chain_id are both required (or pass gnoweb_url to discover them)",
		}
	}

	// Advisory fail-fast: init names are immutable for the whole process
	// lifetime, so a name that passes here cannot become invalid before the
	// authoritative re-check inside AddDynamicProfile.
	if err := s.CheckDynamicProfileName(name); err != nil {
		return server.Result{}, nameToolError(name, err)
	}

	source := "explicit"
	if gnowebURL != "" {
		source = "gnoweb"
		conn, err := gnoweb.Discover(gnowebClient, gnowebURL)
		if err != nil {
			return server.Result{}, &server.ToolError{
				Code:    "gnoweb_discovery_failed",
				Message: fmt.Sprintf("could not discover chain info from %q: %v — pass rpc_url and chain_id explicitly instead", gnowebURL, err),
			}
		}
		// Bound page-derived values once at the source: a malicious gnoweb page
		// can stuff up to the tokenizer cap (~1 MiB) into a meta-tag, and the
		// error paths below echo these values into Message/Extra. Legitimate
		// values are far shorter, and the success path re-validates via regex.
		conn.RPC = truncate(conn.RPC, 256)
		conn.ChainID = truncate(conn.ChainID, 64)
		if gnowebRPCUnusable(gnowebURL, conn.RPC) {
			return server.Result{}, &server.ToolError{
				Code: "gnoweb_rpc_unusable",
				Message: fmt.Sprintf("gnoweb %q advertises RPC %q, a loopback address that cannot be the remote chain — "+
					"the deployment is misconfigured; pass rpc_url and chain_id explicitly", gnowebURL, conn.RPC),
				Extra: map[string]any{"advertised_rpc": conn.RPC, "chain_id": conn.ChainID},
			}
		}
		rpcURL, chainID = conn.RPC, conn.ChainID
	}
	p.RPCURL, p.ChainID = rpcURL, chainID

	// Field-precise checks first (exact error codes), then full Validate as
	// the backstop for everything else (faucet URLs, ...). All of this runs
	// before any network I/O.
	if !profiles.ValidRPCURL(p.RPCURL) {
		return server.Result{}, &server.ToolError{
			Code:    "invalid_rpc_url",
			Message: fmt.Sprintf("rpc_url %q must be an absolute http(s) URL with no spaces or shell metacharacters", p.RPCURL),
		}
	}
	if !profiles.ChainIDValid(p.ChainID) {
		return server.Result{}, &server.ToolError{
			Code:    "chain_id_malformed",
			Message: fmt.Sprintf("chain-id %q is malformed (want lowercase alphanumeric with '.', '-', '_', ≤64 chars)", p.ChainID),
			Extra:   map[string]any{"chain_id": p.ChainID},
		}
	}
	// tx_indexer_url is interpolated into the paste-ready persist_command, so
	// it must pass the same shell-safe gate as rpc_url (Validate does not
	// check it — toml-loaded indexer URLs never reach a shell).
	if p.TxIndexerURL != "" && !profiles.ValidRPCURL(p.TxIndexerURL) {
		return server.Result{}, &server.ToolError{
			Code:    "invalid_indexer_url",
			Message: fmt.Sprintf("tx_indexer_url %q must be an absolute http(s) URL with no spaces or shell metacharacters", p.TxIndexerURL),
		}
	}
	candidate := &profiles.Config{Profiles: map[string]profiles.Profile{name: p}}
	if _, err := candidate.Validate(); err != nil {
		return server.Result{}, &server.ToolError{
			Code:    "invalid_config",
			Message: err.Error(),
		}
	}

	vctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()
	reported, err := verify(vctx, p.RPCURL)
	if err != nil {
		return server.Result{}, &server.ToolError{
			Code:    "chain_unreachable",
			Message: fmt.Sprintf("could not query %q for its chain-id: %v — check the rpc_url (the profile was NOT added)", p.RPCURL, err),
		}
	}
	if reported != p.ChainID {
		// The reported id comes from an arbitrary node; bound it so a
		// malicious node cannot stuff megabytes into the error/Extra channel.
		return server.Result{}, &server.ToolError{
			Code:    "chain_id_mismatch",
			Message: fmt.Sprintf("node at %q reports chain-id %q, not the declared %q (the profile was NOT added)", p.RPCURL, truncate(reported, 64), p.ChainID),
			Extra:   map[string]any{"declared": p.ChainID, "reported": truncate(reported, 64)},
		}
	}

	if err := s.AddDynamicProfile(name, p); err != nil {
		return server.Result{}, nameToolError(name, err)
	}

	if err := onAdded(); err != nil {
		return server.Result{}, &server.ToolError{
			Code: "republish_failed",
			Message: fmt.Sprintf("profile %q was added and is usable by name right away, but re-publishing the tool list failed: %v — "+
				"clients may show stale tool schemas until restart", name, err),
			Extra: map[string]any{"name": name},
		}
	}

	persistCmd := fmt.Sprintf("gnomcp profile add %s --rpc %s --chain-id %s", name, p.RPCURL, p.ChainID)
	if p.TxIndexerURL != "" {
		persistCmd += " --indexer-url " + p.TxIndexerURL
	}
	readOnly := p.IsReadOnly()
	text := fmt.Sprintf("Profile %q added (chain-id %s, RPC %s, via %s).\n"+
		"It is in-memory only and disappears on restart. To persist it, run:\n\n```\n%s\n```\n\n"+
		"Use it now: pass profile=%s to the tools; the tool list was refreshed (tools/list_changed) — "+
		"re-fetch tool schemas if your client cached the old profile list.",
		name, p.ChainID, p.RPCURL, source, persistCmd, name)
	if readOnly {
		text += "\n\nThis is a read-only chain (mainnet/betanet): pass it to the read tools " +
			"(gno_read, gno_packages, gno_render, gno_eval) — there is no agent key, faucet, or write path."
	}
	if p.FaucetServiceURL != "" || p.FaucetURL != "" {
		text += "\n\n(faucet-url / faucet-service-url have no CLI flags — add them to profiles.toml by hand when persisting.)"
	}
	return server.Result{
		Text: text,
		StructuredContent: map[string]any{
			"name":            name,
			"chain_id":        p.ChainID,
			"rpc_url":         p.RPCURL,
			"source":          source,
			"persisted":       false,
			"persist_command": persistCmd,
			"read_only":       readOnly,
		},
	}, nil
}

// nameToolError maps the Server's dynamic-profile name sentinels to
// structured tool errors.
func nameToolError(name string, err error) error {
	code := "invalid_profile_name"
	switch {
	case errors.Is(err, server.ErrProfileReserved):
		code = "profile_reserved"
	case errors.Is(err, server.ErrProfileImmutable):
		code = "profile_immutable"
	}
	return &server.ToolError{Code: code, Message: err.Error(), Extra: map[string]any{"name": name}}
}

// gnowebRPCUnusable reports whether a gnoweb page advertises a loopback (or
// unspecified) RPC host while the page itself is not on loopback. A remote
// gnoweb advertising 127.0.0.1 is a misconfigured deployment (observed live),
// and dialing the agent's own localhost on a remote page's say-so is not
// acceptable. A loopback gnoweb (local gnodev) advertising a loopback RPC is
// the normal local setup and allowed.
func gnowebRPCUnusable(gnowebURL, rpcURL string) bool {
	return hostIsLoopback(rpcURL) && !hostIsLoopback(gnowebURL)
}

// truncate bounds s to max bytes, marking the cut.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…(truncated)"
}

func hostIsLoopback(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}
