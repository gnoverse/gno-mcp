package tools

import (
	"context"
	"encoding/json"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerKeygen) }

func registerKeygen(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_keygen",
		mcp.WithDescription("Generate a new Gno key pair. Never displays or stores a mnemonic. Only returns address and public key. For key backup, direct users to `gnokey export`."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Key name to use in the local keystore")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("name", "")

		addr, pubkey, err := d.Client.Keygen(ctx, name)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_keygen", Result: "err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		_ = d.Audit.Append(audit.Entry{
			Tool:   "gno_keygen",
			Result: "ok",
			Args:   map[string]any{"name": name, "pubkey": pubkey},
		})

		result := map[string]string{
			"name":    name,
			"address": addr,
			"pubkey":  pubkey,
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})
}
