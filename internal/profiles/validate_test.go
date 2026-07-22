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

func TestValidate_nonDevChainIDIsTestnet(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[mystery]
rpc-url = "https://rpc.example/"
chain-id = "test5"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.NoError(t, err)
	assert.True(t, cfg.Profiles["mystery"].IsTestnet(), "non-dev chain-id must derive as testnet")
}

func TestValidate_rejectsEmptyProfileSet(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	_, err := cfg.Validate()
	require.Error(t, err, "expected error for empty profile set")
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

// Sunset is advisory: a retiring testnet stays writable, so a master-address
// on it is a valid configuration (session writes keep working there).
func TestValidate_acceptsMasterAddressOnSunsetProfile(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[oldnet]
rpc-url = "https://rpc.old.example:443"
chain-id = "test-13"
sunset = true
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`))
	require.NoError(t, err)
	_, err = cfg.Validate()
	require.NoError(t, err, "master-address on a sunset (still writable) profile must validate")
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

// ChainIDWritable separates write-capable chains (local dev, known testnets)
// from everything else. Only these get an agent key path and writable tools.
// Testnet names match bare and hyphenated forms alike (test5, test-13, topaz-1).
func TestChainIDWritable(t *testing.T) {
	cases := map[string]bool{
		"dev": true, "test5": true, "test-13": true, "topaz-1": true, "topaz1": true,
		"gnoland1": false, "staging": false, "mychain": false, "portal-loop": false,
		"devnet": false,
	}
	for id, want := range cases {
		assert.Equal(t, want, ChainIDWritable(id), "ChainIDWritable(%q)", id)
	}
}

// ChainIDValid is the format-safety gate: the chain-id is interpolated into the
// `gnomcp profile add` / gnokey commands the user pastes, so shell metacharacters
// and whitespace must be refused even though the WRITE allowlist is relaxed.
func TestChainIDValid(t *testing.T) {
	for _, id := range []string{"dev", "test-13", "gnoland1", "staging", "portal-loop", "a.b-c_1"} {
		assert.True(t, ChainIDValid(id), "ChainIDValid(%q) should be true", id)
	}
	for _, id := range []string{"", "UP", "a b", "x;rm", "a$(b)", "`id`", "-lead", ".lead", "test-", "staging."} {
		assert.False(t, ChainIDValid(id), "ChainIDValid(%q) should be false", id)
	}
}

// A non-test chain-id (betanet/mainnet/staging) is admitted, but read-only:
// auditing deployed source on gno.land requires reaching its chain.
func TestValidate_admitsReadOnlyChains(t *testing.T) {
	for _, id := range []string{"gnoland1", "staging", "mychain"} {
		cfg := &Config{Profiles: map[string]Profile{
			"p": {RPCURL: "https://rpc.example:443", ChainID: id},
		}}
		_, err := cfg.Validate()
		require.NoError(t, err, "non-test chain-id %q must be admitted read-only", id)
		assert.True(t, cfg.Profiles["p"].IsReadOnly(), "%q must classify read-only", id)
		assert.Equal(t, "read-only", cfg.Profiles["p"].Kind())
	}
}

// dev and numbered testnets stay write-capable.
func TestValidate_writableChainsKeepKind(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"l": {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"t": {RPCURL: "https://rpc.example:443", ChainID: "test5"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	assert.False(t, cfg.Profiles["l"].IsReadOnly())
	assert.False(t, cfg.Profiles["t"].IsReadOnly())
}

// Even with the write allowlist relaxed, a chain-id carrying shell
// metacharacters or whitespace must be rejected outright.
func TestValidate_rejectsMalformedChainID(t *testing.T) {
	for _, id := range []string{"a b", "up;rm", "x$(id)", "UP"} {
		cfg := &Config{Profiles: map[string]Profile{
			"p": {RPCURL: "https://rpc.example:443", ChainID: id},
		}}
		_, err := cfg.Validate()
		require.Error(t, err, "malformed chain-id %q must be rejected", id)
		assert.Contains(t, err.Error(), "chain-id")
	}
}

// master-address enables session writes; on a read-only chain that is a
// contradiction, so it is refused rather than silently ignored.
func TestValidate_rejectsMasterAddressOnReadOnlyChain(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"betanet": {
			RPCURL:        "https://rpc.betanet.testnets.gno.land:443",
			ChainID:       "gnoland1",
			MasterAddress: "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3",
		},
	}}
	_, err := cfg.Validate()
	require.Error(t, err, "master-address on a read-only chain must be rejected")
	assert.Contains(t, err.Error(), "read-only")
}

func TestValidate_MasterAddressOptional(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"testnet": {RPCURL: "https://rpc.test13.testnets.gno.land:443", ChainID: "test-13"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err, "read-only profile should validate")
}

func TestValidate_BypassRequiresMaster(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"p": {RPCURL: "https://rpc.test13.testnets.gno.land:443", ChainID: "test-13", BypassHardLimits: true},
	}}
	_, err := cfg.Validate()
	require.Error(t, err, "bypass-hard-limits without master-address should be rejected")
}

func TestValidate_BypassWithMasterAccepted(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"p": {
			RPCURL:           "https://rpc.test13.testnets.gno.land:443",
			ChainID:          "test-13",
			BypassHardLimits: true,
			MasterAddress:    "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3",
		},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err, "bypass + master-address should be accepted")
}

func TestValidate_rejectsInjectionInRPCURL(t *testing.T) {
	// The persisted rpc-url is interpolated into gnokey commands the user
	// pastes into a terminal (session create/revoke), and `profile add
	// --from-gnoweb` copies it verbatim from a remote page's meta-tag — so
	// Validate is the trust boundary that must reject shell metacharacters.
	cases := map[string]string{
		"command substitution": "http://h/$(whoami)",
		"semicolon":            "http://h/;id",
		"newline":              "http://h/\nrm -rf ~",
		"backtick":             "http://h/`id`",
		"space":                "http://h/a b",
		"non-http scheme":      "ftp://h/x",
		"userinfo":             "http://evil@h/x",
	}
	for name, val := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{Profiles: map[string]Profile{
				"p": {RPCURL: val, ChainID: "dev"},
			}}
			_, err := cfg.Validate()
			require.Error(t, err, "rpc-url %q must be rejected", val)
			assert.Contains(t, err.Error(), "rpc-url")
		})
	}
}

func TestValidate_acceptsMixedCaseRPCHost(t *testing.T) {
	// DNS hostnames are case-insensitive and shell-safe in any case.
	cfg := &Config{Profiles: map[string]Profile{
		"p": {RPCURL: "http://MyNode.example:26657", ChainID: "dev"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err, "a mixed-case hostname is a valid, shell-safe rpc-url")
}

func TestValidate_rejectsBadFaucetURL(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"testnet5": {RPCURL: "https://rpc.example:443", ChainID: "test5", FaucetURL: "not a url"},
	}}
	_, err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "faucet-url")
}

func TestValidate_rejectsNonHTTPFaucetURL(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"testnet5": {RPCURL: "https://rpc.example:443", ChainID: "test5", FaucetServiceURL: "ftp://host/x"},
	}}
	_, err := cfg.Validate()
	require.Error(t, err, "non-http(s) scheme must be rejected")
	assert.Contains(t, err.Error(), "faucet-service-url")
}

func TestValidate_LocalityDerivation(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"local":   {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"testnet": {RPCURL: "https://rpc.test13.testnets.gno.land:443", ChainID: "test-13"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	assert.True(t, cfg.Profiles["local"].IsLocal(), "dev chain-id must derive local")
	assert.True(t, cfg.Profiles["testnet"].IsTestnet(), "test-13 chain-id must derive testnet")
}
