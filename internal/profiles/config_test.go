package profiles

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_validProfiles(t *testing.T) {
	src := `
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"

[testnet5]
rpc-url = "https://rpc.test5.gno.land:443"
chain-id = "test5"
tx-indexer-url = "https://indexer.test5.gno.land/graphql/query"
`
	cfg, err := Load(strings.NewReader(src))
	require.NoError(t, err)
	require.Len(t, cfg.Profiles, 2)

	local, ok := cfg.Profiles["local"]
	require.True(t, ok, "missing local profile")
	assert.True(t, local.IsLocal(), "dev profile must derive local")
	assert.Equal(t, "dev", local.ChainID, "local ChainID mis-parsed")
	assert.Equal(t, "http://127.0.0.1:26657", local.RPCURL, "local.RPCURL mis-parsed")

	testnet := cfg.Profiles["testnet5"]
	assert.Equal(t, "https://rpc.test5.gno.land:443", testnet.RPCURL, "testnet5.RPCURL mis-parsed")
	assert.NotEmpty(t, testnet.TxIndexerURL, "testnet5 should have tx-indexer-url set")
}

func TestLoad_malformedTOML(t *testing.T) {
	src := `[local
rpc-url = "http://127.0.0.1:26657"
`
	_, err := Load(strings.NewReader(src))
	require.Error(t, err, "expected error for malformed TOML")
}

func TestLoad_parsesWriteAuthFields(t *testing.T) {
	src := `
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
default-spend-limit = "1000000ugnot"
default-expires-in = "4h"
bypass-hard-limits = true
`
	cfg, err := Load(strings.NewReader(src))
	require.NoError(t, err)

	p := cfg.Profiles["local"]
	assert.Equal(t, "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3", p.MasterAddress)
	assert.Equal(t, "1000000ugnot", p.DefaultSpendLimit)
	assert.Equal(t, "4h", p.DefaultExpiresIn)
	assert.True(t, p.BypassHardLimits, "expected bypass-hard-limits=true")
}

func TestBuiltinProfiles_AllowlistAndShape(t *testing.T) {
	cfg := &Config{Profiles: BuiltinProfiles()}
	_, err := cfg.Validate()
	require.NoError(t, err, "built-in defaults must validate")

	local, ok := cfg.Profiles["local"]
	if !ok || local.ChainID != "dev" {
		assert.Fail(t, "local default missing or wrong chain-id", "%+v", local)
	}
	tn, ok := cfg.Profiles["testnet"]
	require.True(t, ok, "testnet default missing")
	assert.Equal(t, "onyx-1", tn.ChainID, "testnet default chain-id")
	assert.Equal(t, "https://rpc.onyx.testnets.gno.land:443", tn.RPCURL, "testnet default rpc-url")
	assert.Equal(t, "https://onyx.testnets.gno.land", tn.GnowebURL, "testnet default gnoweb-url")
	assert.Equal(t, "https://indexer.onyx.testnets.gno.land/graphql/query", tn.TxIndexerURL, "testnet default tx-indexer-url")
	assert.Equal(t, "https://faucet-agent.onyx.testnets.gno.land", tn.FaucetServiceURL, "testnet default agent-faucet service url")
	assert.True(t, tn.IsTestnet(), "the default testnet must be writable")
	assert.False(t, tn.Sunset, "the default testnet is not retiring")
	assert.Empty(t, local.MasterAddress, "built-in local must be read-only (no master-address)")
	assert.Empty(t, tn.MasterAddress, "built-in testnet must be read-only (no master-address)")

	// mainnet carries real value. It ships the read paths and nothing that
	// could sign.
	main, ok := cfg.Profiles["mainnet"]
	require.True(t, ok, "mainnet default missing")
	assert.Equal(t, "gnoland-1", main.ChainID, "mainnet chain-id")
	assert.True(t, main.IsReadOnly(), "mainnet must be read-only")
	assert.False(t, main.IsTestnet(), "mainnet must not count as a writable testnet")
	assert.False(t, main.Sunset, "read-only is not a sunset label")
	assert.Equal(t, "https://rpc.gno.land:443", main.RPCURL, "mainnet rpc-url")
	assert.Equal(t, "https://gno.land", main.GnowebURL, "mainnet gnoweb-url")
	assert.Equal(t, "https://indexer.gno.land/graphql/query", main.TxIndexerURL, "mainnet indexer serves gnoland-1 and is current")
	assert.Empty(t, main.FaucetServiceURL, "mainnet ships with no faucet")
	assert.Empty(t, main.FaucetURL, "mainnet ships with no faucet")
	assert.Empty(t, main.MasterAddress, "built-in mainnet must carry no master-address")

	// A chain leaves the builtins when reaching it stops being useful: the
	// hosts for betanet, sapphire, topaz, pearl and test13 no longer resolve,
	// so a zero-config profile would offer a chain every call fails against.
	for _, name := range []string{"betanet", "sapphire", "topaz", "pearl", "test13"} {
		_, ok = cfg.Profiles[name]
		assert.False(t, ok, "%q must not ship as a builtin", name)
	}
}

// gnoland-1 is mainnet; gnoland1, one hyphen away, is a retired chain. Neither
// may ever reach a write path, and this pins that rather than leaving it to the
// prefix gate happening not to match.
func TestGnolandChainIDsStayReadOnly(t *testing.T) {
	for _, id := range []string{"gnoland-1", "gnoland1"} {
		t.Run(id, func(t *testing.T) {
			p := Profile{RPCURL: "https://rpc.example:443", ChainID: id}
			assert.False(t, ChainIDWritable(id), "must not be writable")
			assert.True(t, p.IsReadOnly(), "must be read-only")
			assert.False(t, p.IsTestnet(), "must not be a testnet")
			assert.False(t, p.IsLocal(), "must not be local")

			cfg := &Config{Profiles: map[string]Profile{"m": {
				RPCURL: "https://rpc.example:443", ChainID: id,
				MasterAddress: "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5",
			}}}
			_, err := cfg.Validate()
			require.Error(t, err, "master-address must be refused on a read-only chain")
			assert.Contains(t, err.Error(), "read-only")

			for _, faucet := range []Profile{
				{RPCURL: "https://rpc.example:443", ChainID: id, FaucetServiceURL: "https://faucet.example"},
				{RPCURL: "https://rpc.example:443", ChainID: id, FaucetURL: "https://faucet.example/page"},
			} {
				cfg := &Config{Profiles: map[string]Profile{"m": faucet}}
				_, err := cfg.Validate()
				require.Error(t, err, "a faucet on a read-only chain must be refused, not silently ignored")
				assert.Contains(t, err.Error(), "read-only")
			}
		})
	}
}

func TestLoad_parsesFaucetFields(t *testing.T) {
	src := `
[testnet5]
rpc-url = "https://rpc.test5.gno.land:443"
chain-id = "test5"
faucet-url = "https://faucet.test5.gno.land"
faucet-service-url = "http://127.0.0.1:8590"
`
	cfg, err := Load(strings.NewReader(src))
	require.NoError(t, err)
	p := cfg.Profiles["testnet5"]
	assert.Equal(t, "https://faucet.test5.gno.land", p.FaucetURL)
	assert.Equal(t, "http://127.0.0.1:8590", p.FaucetServiceURL)
}

func TestProfile_LocalityDerivedFromChainID(t *testing.T) {
	local := Profile{ChainID: "dev"}
	assert.True(t, local.IsLocal(), "chain-id dev must be local")
	assert.False(t, local.IsTestnet())
	assert.Equal(t, "local", local.Kind())

	tn := Profile{ChainID: "test-13"}
	assert.False(t, tn.IsLocal())
	assert.True(t, tn.IsTestnet(), "chain-id test-13 must be testnet")
	assert.Equal(t, "testnet", tn.Kind())
}

// Sunset is an advisory lifecycle label, not a capability gate: a retiring
// testnet stays fully writable (deploys must work on it without friction) —
// the label only steers new work toward the current testnet.
func TestProfile_sunsetStaysWritable(t *testing.T) {
	p := Profile{ChainID: "test-13", Sunset: true}
	assert.True(t, p.IsTestnet(), "sunset testnet must remain write-capable")
	assert.False(t, p.IsReadOnly())
	assert.Equal(t, "testnet", p.Kind())
}

func TestProfile_RealmViewURL(t *testing.T) {
	const pkg = "gno.land/r/g1abc/tictactoe"
	tests := []struct {
		name    string
		profile Profile
		pkgPath string
		want    string
	}{
		{
			name:    "configured gnoweb-url",
			profile: Profile{GnowebURL: "https://test13.testnets.gno.land", RPCURL: "https://rpc.test13.testnets.gno.land:443"},
			pkgPath: pkg,
			want:    "https://test13.testnets.gno.land/r/g1abc/tictactoe",
		},
		{
			name:    "configured gnoweb-url wins over rpc derivation",
			profile: Profile{GnowebURL: "https://gnoweb.example.com", RPCURL: "https://rpc.test13.testnets.gno.land:443"},
			pkgPath: pkg,
			want:    "https://gnoweb.example.com/r/g1abc/tictactoe",
		},
		{
			name:    "configured gnoweb-url trailing slash trimmed",
			profile: Profile{GnowebURL: "https://test13.testnets.gno.land/"},
			pkgPath: pkg,
			want:    "https://test13.testnets.gno.land/r/g1abc/tictactoe",
		},
		{
			name:    "derived from rpc when gnoweb-url unset",
			profile: Profile{RPCURL: "https://rpc.test13.testnets.gno.land:443"},
			pkgPath: pkg,
			want:    "https://test13.testnets.gno.land/r/g1abc/tictactoe",
		},
		{
			name:    "local node not derivable",
			profile: Profile{RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
			pkgPath: pkg,
			want:    "",
		},
		{
			name:    "no gnoweb-url and no derivable rpc",
			profile: Profile{RPCURL: "http://localhost:26657"},
			pkgPath: pkg,
			want:    "",
		},
		{
			name:    "unspecified host not derivable",
			profile: Profile{RPCURL: "http://0.0.0.0:26657"},
			pkgPath: pkg,
			want:    "",
		},
		{
			name:    "private host not derivable",
			profile: Profile{RPCURL: "http://192.168.1.10:26657"},
			pkgPath: pkg,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.profile.RealmViewURL(tt.pkgPath))
		})
	}
}

func TestLoad_rejectsChainTypeKey(t *testing.T) {
	src := `
[local]
chain-type = "local"
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
`
	_, err := Load(strings.NewReader(src))
	require.Error(t, err, "chain-type is not a config field (locality derives from chain-id); unknown keys must fail loudly")
	assert.Contains(t, err.Error(), "chain-type")
}

func TestMerge_LaterOverridesByName(t *testing.T) {
	base := BuiltinProfiles()
	overlay, err := Load(strings.NewReader(`
[testnet]
rpc-url = "https://rpc.test13.testnets.gno.land:443"
chain-id = "test-13"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`))
	require.NoError(t, err, "load overlay")

	merged := Merge(base, overlay.Profiles)
	assert.NotEmpty(t, merged["testnet"].MasterAddress, "overlay should have added master-address to testnet")
	assert.Contains(t, merged, "local", "base 'local' should survive the merge")
}
