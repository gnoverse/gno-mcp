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
		"local": {ChainType: profiles.ChainTypeLocal, RPCURL: "x", ChainID: "dev"},
		"testnet5": {
			ChainType:     profiles.ChainTypeTestnet,
			RPCURL:        "x",
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
	p := profiles.Profile{ChainType: profiles.ChainTypeTestnet, RPCURL: "x", ChainID: "test5"}
	assert.True(t, profileWritableByAgent(p), "profileWritableByAgent should be true for testnet profiles")
}

func Test_addProfileArg_filtersToSession(t *testing.T) {
	s := newMixedTestServer(t)
	props := map[string]any{}
	var required []string
	addProfileArg(s, props, &required)

	enum := enumFromProps(t, props)
	assert.Equal(t, []string{"testnet5"}, enum, "addProfileArg enum")
}

func Test_addWritableProfileArg_listsBoth(t *testing.T) {
	s := newMixedTestServer(t)
	props := map[string]any{}
	var required []string
	addWritableProfileArg(s, props, &required)

	enum := enumFromProps(t, props)
	assert.Equal(t, []string{"local", "testnet5"}, enum, "addWritableProfileArg enum")
}
