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
chain-type = "local"
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"

[testnet5]
chain-type = "testnet"
rpc-url = "https://rpc.test5.gno.land:443"
chain-id = "test5"
tx-indexer-url = "https://indexer.test5.gno.land/graphql/query"
`
	cfg, err := Load(strings.NewReader(src))
	require.NoError(t, err)
	require.Len(t, cfg.Profiles, 2)

	local, ok := cfg.Profiles["local"]
	require.True(t, ok, "missing local profile")
	assert.Equal(t, "local", local.ChainType, "local ChainType mis-parsed")
	assert.Equal(t, "dev", local.ChainID, "local ChainID mis-parsed")
	assert.Equal(t, "http://127.0.0.1:26657", local.RPCURL, "local.RPCURL mis-parsed")

	testnet := cfg.Profiles["testnet5"]
	assert.Equal(t, "https://rpc.test5.gno.land:443", testnet.RPCURL, "testnet5.RPCURL mis-parsed")
	assert.NotEmpty(t, testnet.TxIndexerURL, "testnet5 should have tx-indexer-url set")
}

func TestLoad_malformedTOML(t *testing.T) {
	src := `[local
chain-type = "local"
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
	if !ok || tn.ChainID != "test11" {
		assert.Fail(t, "testnet default missing or wrong chain-id", "%+v", tn)
	}
	assert.Empty(t, local.MasterAddress, "built-in local must be read-only (no master-address)")
	assert.Empty(t, tn.MasterAddress, "built-in testnet must be read-only (no master-address)")
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

func TestMerge_LaterOverridesByName(t *testing.T) {
	base := BuiltinProfiles()
	overlay, err := Load(strings.NewReader(`
[testnet]
rpc-url = "https://rpc.test11.testnets.gno.land:443"
chain-id = "test11"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`))
	require.NoError(t, err, "load overlay")

	merged := Merge(base, overlay.Profiles)
	assert.NotEmpty(t, merged["testnet"].MasterAddress, "overlay should have added master-address to testnet")
	assert.Contains(t, merged, "local", "base 'local' should survive the merge")
}
