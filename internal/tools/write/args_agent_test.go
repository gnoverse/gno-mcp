package write

import (
	"sort"
	"testing"

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
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return server.NewServer(cfg, "")
}

// enumFromProps extracts and sorts the enum slice from props["profile"].
func enumFromProps(t *testing.T, props map[string]any) []string {
	t.Helper()
	profileProp, ok := props["profile"].(map[string]any)
	if !ok {
		t.Fatal("profile prop missing or wrong type")
	}
	enum, ok := profileProp["enum"].([]string)
	if !ok {
		t.Fatal("enum field missing or wrong type")
	}
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
	if len(enum) != 2 || enum[0] != "local" || enum[1] != "testnet5" {
		t.Errorf("addAgentProfileArg enum = %v, want [local testnet5]", enum)
	}
}

func Test_profileWritableByAgent_testnet(t *testing.T) {
	p := profiles.Profile{ChainType: profiles.ChainTypeTestnet, RPCURL: "x", ChainID: "test5"}
	if !profileWritableByAgent(p) {
		t.Error("profileWritableByAgent should be true for testnet profiles")
	}
}

func Test_addProfileArg_filtersToSession(t *testing.T) {
	s := newMixedTestServer(t)
	props := map[string]any{}
	var required []string
	addProfileArg(s, props, &required)

	enum := enumFromProps(t, props)
	if len(enum) != 1 || enum[0] != "testnet5" {
		t.Errorf("addProfileArg enum = %v, want [testnet5]", enum)
	}
}

func Test_addWritableProfileArg_listsBoth(t *testing.T) {
	s := newMixedTestServer(t)
	props := map[string]any{}
	var required []string
	addWritableProfileArg(s, props, &required)

	enum := enumFromProps(t, props)
	if len(enum) != 2 || enum[0] != "local" || enum[1] != "testnet5" {
		t.Errorf("addWritableProfileArg enum = %v, want [local testnet5]", enum)
	}
}
