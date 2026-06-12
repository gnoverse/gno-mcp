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
//
// Read tools take a free-form string, not an enum: reads reach any configured
// chain (including the read-only mainnet/betanet a connect adds at runtime), so
// a profile added mid-session is usable immediately, without the client
// refetching a regenerated enum. An unknown name errors cleanly at call time.
// The configured profiles are listed in the server instructions; write tools,
// which must gate on chain writability, keep the filtered enum.
func addProfileArg(s *server.Server, props map[string]any, required *[]string) {
	ps := s.ProfileSchema()
	arg := map[string]any{"type": "string"}
	if ps.Default != "" {
		arg["default"] = ps.Default
		arg["description"] = fmt.Sprintf(
			"Target chain profile — any configured profile name (see the server instructions for the list), including one added at runtime via gno_profile_add. Default: %q.", ps.Default)
	} else {
		arg["description"] = "Target chain profile — any configured profile name (see the server instructions for the list), including one added at runtime via gno_profile_add. Required (no default — pick one explicitly)."
	}
	props["profile"] = arg
	if ps.Required {
		*required = append(*required, "profile")
	}
}
