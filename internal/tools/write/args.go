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

// addOptionalKeyArg adds the optional `key` arg that selects which of a profile's
// named agent keys to act with. Omitted ⇒ the "default" key.
func addOptionalKeyArg(props map[string]any) {
	props["key"] = map[string]any{
		"type": "string",
		"description": "Optional name of the agent key to act with (default: \"default\"). " +
			"Applies to identity=agent only — it is rejected with identity=session, which signs with the session key. " +
			"A profile can hold several keys so you can exercise realms involving multiple addresses; " +
			"create more with gno_key_generate and list them with gno_key_list. e.g. \"bob\".",
	}
}

// keyArg reads the optional `key` arg; "" means the default key (the keystore
// resolves it).
func keyArg(args map[string]any) (string, error) {
	return server.StringArg(args, "key")
}

// profileSessionEligible reports whether a chain-bound session can exist for p:
// any writable chain (local/testnet). The master account comes from the
// profile's master-address, or from a master_address the user supplies at
// propose time when the profile has none. Read-only chains can't write at all.
func profileSessionEligible(p profiles.Profile) bool { return profiles.ChainIDWritable(p.ChainID) }

// profileWritableByAgent reports whether the agent has or can have its own
// signing key for p: local (test1 key) and testnet (generated key) chains.
// Read-only chains (mainnet/betanet) have no agent key path, so they are
// excluded from every write tool's profile enum.
func profileWritableByAgent(p profiles.Profile) bool { return profiles.ChainIDWritable(p.ChainID) }

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

// addProfileArg adds the `profile` arg for the session tools, filtered to
// writable chains (local/testnet). A profile with a master-address uses it; a
// writable profile without one takes the master from master_address at propose
// time, so both are session-eligible.
func addProfileArg(s *server.Server, props map[string]any, required *[]string) {
	addProfileArgFiltered(s, props, required, profileSessionEligible,
		"Profile to use for a chain-bound session. Writable chains (local/testnet); a profile without a master-address needs master_address supplied at propose time.")
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
