package write

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// newMixedTestServer builds a Server with both a local profile (no master)
// and a testnet5 profile (with master-address).
func newMixedTestServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local": {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"testnet5": {
			RPCURL:        "http://127.0.0.1:26657",
			ChainID:       "test5",
			MasterAddress: "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3",
		},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err, "validate")
	return server.NewServer(cfg, "")
}

// enumFromProps extracts and sorts the enum slice from props["profile"].
func enumFromProps(t *testing.T, props map[string]any) []string {
	t.Helper()
	profileProp, ok := props["profile"].(map[string]any)
	require.True(t, ok, "profile prop missing or wrong type")
	enum, ok := profileProp["enum"].([]string)
	require.True(t, ok, "enum field missing or wrong type")
	sorted := make([]string, len(enum))
	copy(sorted, enum)
	sort.Strings(sorted)
	return sorted
}

func Test_addAgentProfileArg_filtersToLocal(t *testing.T) {
	s := newMixedTestServer(t)
	props := map[string]any{}
	var required []string
	addAgentProfileArg(s, props, &required)

	enum := enumFromProps(t, props)
	assert.Equal(t, []string{"local", "testnet5"}, enum, "addAgentProfileArg enum")
}

func Test_profileWritableByAgent_testnet(t *testing.T) {
	p := profiles.Profile{RPCURL: "http://127.0.0.1:26657", ChainID: "test5"}
	assert.True(t, profileWritableByAgent(p), "profileWritableByAgent should be true for testnet profiles")
}

// The profile arg's description carries the live name->chain-id map, so an
// agent told "deploy on <chain name>" can resolve the profile without an
// extra lookup.
func Test_addAgentProfileArg_descriptionMapsNamesToChainIDs(t *testing.T) {
	s := newMixedTestServer(t)
	props := map[string]any{}
	var required []string
	addAgentProfileArg(s, props, &required)

	arg := props["profile"].(map[string]any)
	desc := arg["description"].(string)
	assert.Contains(t, desc, "local (chain dev)", "local mapping missing")
	assert.Contains(t, desc, "testnet5 (chain test5)", "testnet mapping missing")
}

// A sunset testnet stays in every write enum: the label is advisory (steer new
// work to the current testnet) — deploys to a retiring chain must still work.
func Test_sunsetProfileStaysInWritePaths(t *testing.T) {
	p := profiles.Profile{RPCURL: "http://127.0.0.1:26657", ChainID: "test5", Sunset: true}
	assert.True(t, profileWritableByAgent(p), "sunset profile must stay agent-writable")
	assert.True(t, profileSessionEligible(p), "sunset profile must stay session-eligible")
}

// The session profile enum lists writable chains (local + testnet) — a profile
// without a master-address is session-eligible too, taking the master from
// master_address at propose time.
func Test_addProfileArg_filtersToSession(t *testing.T) {
	s := newMixedTestServer(t)
	props := map[string]any{}
	var required []string
	addProfileArg(s, props, &required)

	enum := enumFromProps(t, props)
	assert.Equal(t, []string{"local", "testnet5"}, enum, "addProfileArg enum (writable chains)")
}

func Test_addWritableProfileArg_listsBoth(t *testing.T) {
	s := newMixedTestServer(t)
	props := map[string]any{}
	var required []string
	addWritableProfileArg(s, props, &required)

	enum := enumFromProps(t, props)
	assert.Equal(t, []string{"local", "testnet5"}, enum, "addWritableProfileArg enum")
}
