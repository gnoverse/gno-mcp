// Package write holds the chain-write and write-prep MCP tools: gno_call,
// gno_run, gno_addpkg (transactions); gno_faucet_fund, gno_key_generate,
// gno_key_address (agent key lifecycle); and gno_session_propose,
// gno_auth_status, gno_session_revoke (session lifecycle). Each exposes a
// Register* function. All register unconditionally except gno_faucet_fund,
// which is gated on a testnet profile existing; see cmd/gnomcp/register.go.
package write

import (
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// requireProfile resolves the required profile arg to its config entry,
// applying the shared error wording for a missing or unknown profile.
func requireProfile(args map[string]any, s *server.Server) (string, profiles.Profile, error) {
	name, err := server.StringArg(args, "profile")
	if err != nil {
		return "", profiles.Profile{}, err
	}
	if name == "" {
		return "", profiles.Profile{}, fmt.Errorf("profile: required — pick one of the configured profiles")
	}
	p, ok := s.Config().Profiles[name]
	if !ok {
		return "", profiles.Profile{}, fmt.Errorf("profile %q: not found", name)
	}
	return name, p, nil
}

func profileWritableBySession(p profiles.Profile) bool { return p.MasterAddress != "" }

// profileWritableByAgent reports whether the agent has or can have its own
// signing key for p. The chain-id allowlist admits only local (test1 key) and
// testnet (generated key) chains, so every profile qualifies.
func profileWritableByAgent(profiles.Profile) bool { return true }

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

// addAgentProfileArg adds the `profile` arg filtered to profiles where the
// agent can sign — local (test1) and testnet (generated key via gno_key_generate).
func addAgentProfileArg(s *server.Server, props map[string]any, required *[]string) {
	addProfileArgFiltered(s, props, required, profileWritableByAgent,
		"Profile to use. Local profiles use the built-in test1 key; testnet profiles require a generated agent key (gno_key_generate).")
}

func profileIsTestnet(p profiles.Profile) bool { return p.IsTestnet() }

// addTestnetProfileArg adds the `profile` arg filtered to testnet profiles,
// where the agent can generate and persist its own key.
func addTestnetProfileArg(s *server.Server, props map[string]any, required *[]string) {
	addProfileArgFiltered(s, props, required, profileIsTestnet,
		"Profile to use. Only testnet profiles support agent key generation.")
}

// addWritableProfileArg adds the `profile` arg listing all profiles writable
// via an agent key (local or testnet) or a session (master-address).
func addWritableProfileArg(s *server.Server, props map[string]any, required *[]string) {
	addProfileArgFiltered(s, props, required, func(p profiles.Profile) bool {
		return profileWritableByAgent(p) || profileWritableBySession(p)
	}, "Profile to use. Lists profiles writable via an agent key (local/testnet) or a session (master-address).")
}
