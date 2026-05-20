// Package read holds the chain read MCP tools (gno_render, gno_eval,
// gno_read, gno_inspect). Each tool exposes a single Register* function
// that adds itself to a server.Server's Registry.
package read

import (
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/server"
)

// stringArg pulls a typed string from the schema-validated args map.
// Returns ok=false (with an error mentioning the field name) if the
// key is present but not a string. A missing key is treated as the
// empty string with ok=true; required-vs-optional is the caller's
// concern.
func stringArg(args map[string]any, name string) (string, error) {
	raw, present := args[name]
	if !present {
		return "", nil
	}
	v, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s: expected string, got %T", name, raw)
	}
	return v, nil
}

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
