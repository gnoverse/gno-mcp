package read

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gnoverse/gno-mcp/internal/gnoweb"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// RegisterConnect adds the read-only gno_connect tool. client is the HTTP client
// used to fetch gnoweb pages.
func RegisterConnect(s *server.Server, client *http.Client) {
	s.Registry().Add(&server.Tool{
		Name: "gno_connect",
		Description: "Discovers how to connect to a gno chain from its gnoweb URL: reads the " +
			"gnoconnect:rpc and gnoconnect:chainid meta-tags, validates the chain-id against the " +
			"allowlist (only dev or testNN), and returns both follow-up paths — gno_profile_add " +
			"to use the chain in this session (in-memory), and a ready-to-run 'gnomcp profile add' " +
			"command for the user to persist it. Use to PREVIEW a chain's connection info without " +
			"changing gnomcp state; to discover AND add in one step, call gno_profile_add with " +
			"gnoweb_url directly instead. Does NOT modify any config itself. " +
			"Required: gnoweb_url (e.g. 'https://test11.testnets.gno.land'). " +
			"Optional: name (suggested profile name, default derived from chain-id).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"gnoweb_url": map[string]any{
					"type":        "string",
					"format":      "uri",
					"description": "Base gnoweb URL of the chain (e.g. 'https://test11.testnets.gno.land').",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Suggested profile name (e.g. 'test11'). Default: the chain-id.",
				},
			},
			"required":             []string{"gnoweb_url"},
			"additionalProperties": false,
		},
		OutputKind: server.OutputText,
		Capability: server.CapBaseRead,
		Annotations: server.Annotations{
			ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: true,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			url, err := server.StringArg(args, "gnoweb_url")
			if err != nil {
				return server.Result{}, err
			}
			if url == "" {
				return server.Result{}, fmt.Errorf("gnoweb_url is required")
			}
			conn, err := gnoweb.Discover(client, url)
			if err != nil {
				return server.Result{}, fmt.Errorf("gno_connect: %w", err)
			}
			if !profiles.ChainIDAllowed(conn.ChainID) {
				return server.Result{}, &server.ToolError{
					Code:    "chain_forbidden",
					Message: fmt.Sprintf("chain-id %q is not allowed (only dev or testNN); cannot create a profile for it", conn.ChainID),
					Extra:   map[string]any{"chain_id": conn.ChainID, "rpc": conn.RPC},
				}
			}
			// conn.RPC comes verbatim from a meta-tag of an arbitrary fetched page,
			// and name from an LLM-supplied arg — both are interpolated into the
			// command the user is told to paste into a terminal, so both must be
			// shell-safe before the command is built.
			if !profiles.ValidRPCURL(conn.RPC) {
				return server.Result{}, fmt.Errorf("gno_connect: discovered RPC %q is not a safe http(s) URL; refusing to build a paste command from it", conn.RPC)
			}
			name, _ := server.StringArg(args, "name")
			if name == "" {
				name = conn.ChainID
			}
			if !profiles.ValidProfileName(name) {
				return server.Result{}, fmt.Errorf("gno_connect: profile name %q must be lowercase alphanumeric with '-' or '_' (e.g. %q)", name, conn.ChainID)
			}
			cmd := fmt.Sprintf("gnomcp profile add %s --rpc %s --chain-id %s", name, conn.RPC, conn.ChainID)
			text := fmt.Sprintf(
				"Discovered chain %q at RPC %s.\n\n"+
					"To use it in this session (in-memory until restart), call gno_profile_add with "+
					"name=%q, rpc_url=%q, chain_id=%q.\n\n"+
					"To persist it, run:\n\n```\n%s\n```\n\n"+
					"Then pass profile=%q to the read tools. For write-as-user sessions, persist the "+
					"profile with --master <g1...> appended (dynamic profiles support agent-key writes only).",
				conn.ChainID, conn.RPC, name, conn.RPC, conn.ChainID, cmd, name)
			return server.Result{
				Text: text,
				StructuredContent: map[string]any{
					"rpc": conn.RPC, "chain_id": conn.ChainID, "name": name, "command": cmd,
				},
			}, nil
		},
	})
}
