package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// The instructions must steer the model to enumerate the chain before guessing
// package paths or asking the user — "bump the counter" should resolve via
// gno_packages, not via invalid-path roulette.
func TestServerInstructions_steerDiscoveryBeforeGuessing(t *testing.T) {
	assert.Contains(t, serverInstructions, "DISCOVER")
	assert.Contains(t, serverInstructions, "gno_packages")
	assert.Contains(t, serverInstructions, "guess")
}

// Tool errors carry repair instructions; when the user's request already
// authorizes the repair, the model should perform it rather than relaying the
// instructions back to the user.
func TestServerInstructions_steerAuthorizedRepair(t *testing.T) {
	assert.Contains(t, serverInstructions, "RECOVER")
	assert.Contains(t, serverInstructions, "repair")
}

// "Act as the user" pattern-matches to "need the user's private key" in
// generic blockchain priors; the instructions must preempt that — keys never
// enter the conversation, the session flow is the answer.
func TestServerInstructions_keyMaterialNeverRequested(t *testing.T) {
	assert.Contains(t, serverInstructions, "IDENTITY")
	assert.Contains(t, serverInstructions, "key material")
}

// A gnoweb URL names a SPECIFIC chain. The agent must resolve the chain from
// the URL (gno_profile_add) before reading — not query whatever profile is
// ambient. This is the rule that stops "audit https://gno.land/..." from being
// looked up on a local sim and then falling back to off-chain source.
func TestServerInstructions_urlNamesTheChain(t *testing.T) {
	assert.Contains(t, serverInstructions, "gnoweb URL")
	assert.Contains(t, serverInstructions, "gno_profile_add")
	assert.Contains(t, serverInstructions, "ambient")
}

// At startup the agent should already know which profiles exist and what chain
// each targets, without calling a tool first — so the configured profiles are
// listed in the initialize-time instructions. The base guidance is kept.
func TestBuildServerInstructions_listsConfiguredProfiles(t *testing.T) {
	got := buildServerInstructions(map[string]profiles.Profile{
		"testnet": {RPCURL: "http://testnet.gnomcp.sim:26687", ChainID: "test9999"},
		"local":   {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"mychain": {RPCURL: "https://rpc.example", ChainID: "test5", MasterAddress: "g1abc"},
	})
	assert.Contains(t, got, "DISCOVER", "base guidance must still be present")
	for _, want := range []string{
		"testnet", "test9999", "http://testnet.gnomcp.sim:26687",
		"local", "dev", "http://127.0.0.1:26657",
		"mychain", "test5",
	} {
		assert.Contains(t, got, want)
	}
	// a master-bearing profile is flagged as write-as-user capable
	assert.Contains(t, got, "write-as-user")
}

// A read-only chain (mainnet/betanet) supports read tools only. The startup
// listing must flag it so the agent doesn't attempt writes against it.
func TestBuildServerInstructions_flagsReadOnlyProfile(t *testing.T) {
	got := buildServerInstructions(map[string]profiles.Profile{
		"betanet": {RPCURL: "https://rpc.betanet.testnets.gno.land", ChainID: "gnoland1"},
	})
	assert.Contains(t, got, "read tools only")
}

// The agent needs to know where realms are viewable, so a profile's gnoweb URL
// is surfaced — that's what it tells the user instead of guessing gno.land.
func TestBuildServerInstructions_showsGnowebURL(t *testing.T) {
	got := buildServerInstructions(map[string]profiles.Profile{
		"testnet": {RPCURL: "http://x:26687", ChainID: "test9999", GnowebURL: "http://localhost:8688"},
	})
	assert.Contains(t, got, "http://localhost:8688")
}

// Sessions authorize gno_call/gno_run only — gno_addpkg is agent-key-only — so
// the instructions must not let the model promise deploy-as-the-user via a
// session. Deploying as the user means the user runs gnokey themselves.
func TestServerInstructions_sessionsDoNotCoverDeploy(t *testing.T) {
	assert.Contains(t, serverInstructions, "gno_addpkg")
	assert.Contains(t, serverInstructions, "not session")
}

// A master-less writable profile must not stall the agent into editing config:
// the session flow asks the user for their PUBLIC address (master_address).
func TestServerInstructions_sessionAsksForPublicMaster(t *testing.T) {
	assert.Contains(t, serverInstructions, "master_address")
	assert.Contains(t, serverInstructions, "PUBLIC")
}

// Deterministic ordering keeps the prompt-cache stable across restarts.
func TestBuildServerInstructions_deterministicOrder(t *testing.T) {
	profs := map[string]profiles.Profile{
		"zeta":  {RPCURL: "http://z", ChainID: "test9"},
		"alpha": {RPCURL: "http://a", ChainID: "test8"},
	}
	assert.Equal(t, buildServerInstructions(profs), buildServerInstructions(profs))
	// alpha is listed before zeta (sorted by name)
	got := buildServerInstructions(profs)
	assert.Less(t, indexOf(got, "alpha"), indexOf(got, "zeta"))
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
