// Package read holds the chain read MCP tools (gno_render, gno_eval,
// gno_read, gno_packages, gno_account, gno_status, gno_connect). Each tool
// exposes a single Register* function that adds itself to a server.Server's
// Registry.
package read

import (
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/server"
)

// addProfileArg adds the `profile` arg to the props map per the server's
// schema rules. Shared across all chain-bound tools in this package.
func addProfileArg(s *server.Server, props map[string]any, required *[]string) {
	ps := s.ProfileSchema()
	arg := map[string]any{
		"type": "string",
		"enum": ps.Enum,
	}
	if ps.Default != "" {
		arg["default"] = ps.Default
		arg["description"] = fmt.Sprintf("Target chain profile. Default: %q.", ps.Default)
	} else {
		arg["description"] = "Target chain profile. Required (no default — pick one explicitly)."
	}
	props["profile"] = arg
	if ps.Required {
		*required = append(*required, "profile")
	}
}
