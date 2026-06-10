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
		Description: "Discovers how to connect to a gno chain from its gnoweb URL and returns a " +
			"ready-to-run 'gnomcp profile add' command for the user. Use when the user wants to " +
			"read a chain that is not in the current profile list. Reads the gnoconnect:rpc and " +
			"gnoconnect:chainid meta-tags from the gnoweb page; validates the chain-id against the " +
			"allowlist (only dev or testNN). Does NOT modify any config — it only tells the user " +
			"the command to run. Required: gnoweb_url (e.g. 'https://test11.testnets.gno.land'). " +
			"Optional: name (suggested profile name, default derived from chain-id).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"gnoweb_url": map[string]any{
					"type":        "string",
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
				"Discovered chain %q at RPC %s.\n\nTo add it as a profile, run:\n\n```\n%s\n```\n\n"+
					"Then pass profile=%q to the read tools. To enable writes, append --master <g1...>.",
				conn.ChainID, conn.RPC, cmd, name)
			return server.Result{
				Text: text,
				StructuredContent: map[string]any{
					"rpc": conn.RPC, "chain_id": conn.ChainID, "name": name, "command": cmd,
				},
			}, nil
		},
	})
}
