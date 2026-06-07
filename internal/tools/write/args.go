// Package write holds the chain-write MCP tools (gno_call, gno_run,
// gno_auth_status, gno_session_propose, gno_session_revoke). Each tool
// exposes a Register* function. All tools register only when at least
// one profile has a master-address set (writable profile).
package write

import (
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/profiles"
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

func profileWritableBySession(p profiles.Profile) bool { return p.MasterAddress != "" }
func profileWritableByAgent(p profiles.Profile) bool   { return p.ChainType == profiles.ChainTypeLocal }

// addProfileArgFiltered populates props["profile"] with an enum filtered by keep.
// desc is the human-readable description for the arg.
func addProfileArgFiltered(s *server.Server, props map[string]any, required *[]string, keep func(profiles.Profile) bool, desc string) {
	ps := s.ProfileSchema()
	cfg := s.Config()
	enum := []string{}
	for _, name := range ps.Enum {
		if keep(cfg.Profiles[name]) {
			enum = append(enum, name)
		}
	}
	arg := map[string]any{"type": "string", "enum": enum, "description": desc}
	if ps.Default != "" && keep(cfg.Profiles[ps.Default]) {
		arg["default"] = ps.Default
	}
	props["profile"] = arg
	if ps.Required {
		*required = append(*required, "profile")
	}
}

// addProfileArg adds the `profile` arg to props, filtered to profiles
// with a master-address set (session-writable profiles). If no such profile
// exists the enum is empty, so the agent has no writable target to pick.
func addProfileArg(s *server.Server, props map[string]any, required *[]string) {
	addProfileArgFiltered(s, props, required, profileWritableBySession,
		"Profile to use. Only profiles with a master-address (session-writable) are listed.")
}

// addAgentProfileArg adds the `profile` arg filtered to local (dev) profiles
// where the agent has a test1 key available.
func addAgentProfileArg(s *server.Server, props map[string]any, required *[]string) {
	addProfileArgFiltered(s, props, required, profileWritableByAgent,
		"Profile to use. Only local (dev) profiles with an agent key are listed.")
}

// addWritableProfileArg adds the `profile` arg listing all profiles writable
// via an agent key (local) or a session (master-address).
func addWritableProfileArg(s *server.Server, props map[string]any, required *[]string) {
	addProfileArgFiltered(s, props, required, func(p profiles.Profile) bool {
		return profileWritableByAgent(p) || profileWritableBySession(p)
	}, "Profile to use. Lists profiles writable via an agent key (local) or a session (master-address).")
}
