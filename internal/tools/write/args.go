// Package write holds the chain-write MCP tools (gno_call, gno_run,
// gno_auth_status, gno_session_propose, gno_session_revoke). Each tool
// exposes a Register* function. All tools register only when at least
// one profile has allow-dangerous-tools=true.
package write

import (
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/server"
)

// stringArg pulls a typed string from the schema-validated args map.
// Missing key returns ("", nil); required-vs-optional is the caller's
// concern. Present but wrong type returns an error.
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

// boolArg pulls a typed bool from the args map.
// Missing key returns (false, nil). Present but wrong type returns an error.
func boolArg(args map[string]any, name string) (bool, error) {
	raw, present := args[name]
	if !present {
		return false, nil
	}
	v, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s: expected bool, got %T", name, raw)
	}
	return v, nil
}

// stringSliceArg pulls a []string from the args map.
// Missing key returns (nil, nil). Present value must be []any with every
// element a string; a non-string element returns an error.
func stringSliceArg(args map[string]any, name string) ([]string, error) {
	raw, present := args[name]
	if !present {
		return nil, nil
	}
	rawSlice, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected array, got %T", name, raw)
	}
	out := make([]string, len(rawSlice))
	for i, elem := range rawSlice {
		s, ok := elem.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d]: expected string, got %T", name, i, elem)
		}
		out[i] = s
	}
	return out, nil
}

// addProfileArg adds the `profile` arg to props, filtered to profiles
// where AllowDangerousTools == true. If no dangerous profile exists
// the enum is empty (tools won't register anyway per the gate in main.go).
func addProfileArg(s *server.Server, props map[string]any, required *[]string) {
	ps := s.ProfileSchema()
	cfg := s.Config()
	var enum []string
	for _, name := range ps.Enum {
		if cfg.Profiles[name].AllowDangerousTools {
			enum = append(enum, name)
		}
	}
	if enum == nil {
		enum = []string{}
	}
	arg := map[string]any{
		"type":        "string",
		"enum":        enum,
		"description": "Profile to use. Only profiles with allow-dangerous-tools=true are listed.",
	}
	if ps.Default != "" && cfg.Profiles[ps.Default].AllowDangerousTools {
		arg["default"] = ps.Default
	}
	props["profile"] = arg
	if ps.Required {
		*required = append(*required, "profile")
	}
}
