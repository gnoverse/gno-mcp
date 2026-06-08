package profiles

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_requiresRPCURL(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
chain-id = "dev"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.Error(t, err, "expected error for missing rpc-url")
}

func TestValidate_requiresChainID(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.Error(t, err, "expected error for missing chain-id")
}

func TestValidate_chainTypeDefaultsToTestnet(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[mystery]
rpc-url = "https://rpc.example/"
chain-id = "test5"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, "testnet", cfg.Profiles["mystery"].ChainType, "expected chain-type=testnet default")
}

func TestValidate_rejectsEmptyProfileSet(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	_, err := cfg.Validate()
	require.Error(t, err, "expected error for empty profile set")
}

func TestValidate_rejectsUnknownChainType(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[weird]
chain-type = "moonchain"
rpc-url = "http://x"
chain-id = "x"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.Error(t, err, "expected error for unknown chain-type")
}

func TestLoad_rejectsUnknownKey(t *testing.T) {
	src := `
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
foo-bar = true
`
	_, err := Load(strings.NewReader(src))
	require.Error(t, err, "expected error for unknown key foo-bar")
	assert.Contains(t, err.Error(), "foo-bar", "error should mention the offending key")
}

func TestValidate_rejectsMalformedDefaultExpiresIn(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-expires-in = "forever"
`))
	require.NoError(t, err)

	_, err = cfg.Validate()
	require.Error(t, err, "expected error for malformed default-expires-in")
	assert.Contains(t, err.Error(), "default-expires-in")
	assert.Contains(t, err.Error(), "forever")
}

func TestValidate_rejectsMalformedDefaultSpendLimit(t *testing.T) {
	cases := map[string]string{
		"letters only (no magnitude)": "abc",
		"denom only (no magnitude)":   "ugnot",
		"digits only (no denom)":      "100",
	}
	for name, val := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := Load(strings.NewReader(fmt.Sprintf(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-spend-limit = %q
`, val)))
			require.NoError(t, err)
			_, err = cfg.Validate()
			require.Error(t, err, "expected error for spend-limit %q", val)
			assert.Contains(t, err.Error(), "default-spend-limit")
		})
	}
}

func TestValidate_acceptsValidExpiresIn(t *testing.T) {
	cases := []string{"0s", "500ms", "2h", "72h30m", "168h"}
	for _, val := range cases {
		t.Run(val, func(t *testing.T) {
			cfg, err := Load(strings.NewReader(fmt.Sprintf(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-expires-in = %q
`, val)))
			require.NoError(t, err)
			_, err = cfg.Validate()
			assert.NoError(t, err, "Validate rejected valid duration %q", val)
		})
	}
}

func TestValidate_acceptsValidWriteFields(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
chain-type = "local"
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
default-spend-limit = "500000ugnot"
default-expires-in = "2h"
bypass-hard-limits = true
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.NoError(t, err)
}

func TestValidate_rejectsMalformedMasterAddress(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
master-address = "not-a-bech32-address"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.Error(t, err, "expected error for malformed master-address")
	assert.Contains(t, err.Error(), "master-address")
}

func TestValidate_rejectsMalformedMasterAddressEvenWhenReadOnly(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
master-address = "not-a-bech32-address"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.Error(t, err, "expected error for malformed master-address on read-only profile")
	assert.Contains(t, err.Error(), "master-address")
	assert.Contains(t, err.Error(), "not-a-bech32-address")
}

func TestValidate_acceptsEmptyMasterAddressWhenReadOnly(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.NoError(t, err, "read-only profile without master-address should validate")
}

func TestValidate_ChainIDAllowlist(t *testing.T) {
	cases := []struct {
		name    string
		chainID string
		wantErr bool
	}{
		{"dev-ok", "dev", false},
		{"test11-ok", "test11", false},
		{"test-13-hyphen-ok", "test-13", false},
		{"betanet-rejected", "gnoland1", true},
		{"staging-rejected", "staging", true},
		{"arbitrary-rejected", "mychain", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Profiles: map[string]Profile{
				"p": {RPCURL: "https://rpc.example:443", ChainID: tc.chainID},
			}}
			_, err := cfg.Validate()
			if tc.wantErr {
				require.Error(t, err, "chain-id %q: expected reject", tc.chainID)
			} else {
				require.NoError(t, err, "chain-id %q: expected ok", tc.chainID)
			}
		})
	}
}

func TestValidate_MasterAddressOptional(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"testnet": {RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err, "read-only profile should validate")
}

func TestValidate_BypassRequiresMaster(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"p": {RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11", BypassHardLimits: true},
	}}
	_, err := cfg.Validate()
	require.Error(t, err, "bypass-hard-limits without master-address should be rejected")
}

func TestValidate_BypassWithMasterAccepted(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"p": {
			RPCURL:           "https://rpc.test11.testnets.gno.land:443",
			ChainID:          "test11",
			BypassHardLimits: true,
			MasterAddress:    "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3",
		},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err, "bypass + master-address should be accepted")
}

func TestValidate_DerivesChainType(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"local":   {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"testnet": {RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, ChainTypeLocal, cfg.Profiles["local"].ChainType, "local chain-type")
	assert.Equal(t, ChainTypeTestnet, cfg.Profiles["testnet"].ChainType, "testnet chain-type")
}
