// Package indexer holds the tx-indexer-backed MCP tools (gno_list,
// gno_history, gno_activity). Each tool exposes a Register* function
// that adds itself to a server.Server's Registry. Tools register with
// Capability=CapIndexerRead and are only wired up when at least one
// profile has tx-indexer-url configured.
package indexer

import (
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/server"
)

// stringArg pulls a typed string from the schema-validated args map.
// Returns ok=false (with an error mentioning the field name) if the
// key is present but not a string. Missing key returns ("", nil);
// required-vs-optional is the caller's concern.
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

// addProfileArg adds the profile arg, restricted to profiles whose
// tx-indexer-url is set. Profiles without an indexer can't serve
// indexer tools, so listing them in the enum would mislead the agent.
func addProfileArg(s *server.Server, props map[string]any, required *[]string) {
	ps := s.ProfileSchema()
	cfg := s.Config()
	var enum []string
	for _, name := range ps.Enum {
		if cfg.Profiles[name].TxIndexerURL != "" {
			enum = append(enum, name)
		}
	}
	arg := map[string]any{
		"type":        "string",
		"enum":        enum,
		"description": "Profile whose tx-indexer-url to query. Only profiles with an indexer configured are listed.",
	}
	if ps.Default != "" && cfg.Profiles[ps.Default].TxIndexerURL != "" {
		arg["default"] = ps.Default
	}
	props["profile"] = arg
	if ps.Required {
		*required = append(*required, "profile")
	}
}
