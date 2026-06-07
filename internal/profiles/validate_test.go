package profiles

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidate_requiresRPCURL(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
chain-id = "dev"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing rpc-url")
	}
}

func TestValidate_requiresChainID(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing chain-id")
	}
}

func TestValidate_chainTypeDefaultsToTestnet(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[mystery]
rpc-url = "https://rpc.example/"
chain-id = "test5"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := cfg.Profiles["mystery"].ChainType; got != "testnet" {
		t.Errorf("expected chain-type=testnet default, got %q", got)
	}
}

func TestValidate_rejectsEmptyProfileSet(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty profile set")
	}
}

func TestValidate_rejectsUnknownChainType(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[weird]
chain-type = "moonchain"
rpc-url = "http://x"
chain-id = "x"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown chain-type")
	}
}

func TestLoad_rejectsUnknownKey(t *testing.T) {
	src := `
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
foo-bar = true
`
	_, err := Load(strings.NewReader(src))
	if err == nil {
		t.Fatal("expected error for unknown key foo-bar, got nil")
	}
	// Must mention the offending key so users can fix their config.
	if !strings.Contains(err.Error(), "foo-bar") {
		t.Errorf("error %q should mention the offending key", err)
	}
}

func TestValidate_rejectsMalformedDefaultExpiresIn(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-expires-in = "forever"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = cfg.Validate()
	if err == nil {
		t.Fatal("expected error for malformed default-expires-in")
	}
	if !strings.Contains(err.Error(), "default-expires-in") {
		t.Errorf("error %q should mention field name", err)
	}
	if !strings.Contains(err.Error(), "forever") {
		t.Errorf("error %q should mention offending value", err)
	}
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
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			_, err = cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for spend-limit %q", val)
			}
			if !strings.Contains(err.Error(), "default-spend-limit") {
				t.Errorf("error %q should mention field name", err)
			}
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
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if _, err := cfg.Validate(); err != nil {
				t.Errorf("Validate rejected valid duration %q: %v", val, err)
			}
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
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}

func TestValidate_rejectsMalformedMasterAddress(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
master-address = "not-a-bech32-address"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = cfg.Validate()
	if err == nil {
		t.Fatal("expected error for malformed master-address")
	}
	if !strings.Contains(err.Error(), "master-address") {
		t.Errorf("error %q should mention master-address", err)
	}
}

func TestValidate_rejectsMalformedMasterAddressEvenWhenReadOnly(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
master-address = "not-a-bech32-address"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = cfg.Validate()
	if err == nil {
		t.Fatal("expected error for malformed master-address on read-only profile")
	}
	if !strings.Contains(err.Error(), "master-address") {
		t.Errorf("error %q should mention master-address", err)
	}
	if !strings.Contains(err.Error(), "not-a-bech32-address") {
		t.Errorf("error %q should mention the offending value", err)
	}
}

func TestValidate_acceptsEmptyMasterAddressWhenReadOnly(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error for read-only profile without master-address: %v", err)
	}
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
			if tc.wantErr && err == nil {
				t.Fatalf("chain-id %q: expected reject, got nil", tc.chainID)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("chain-id %q: expected ok, got %v", tc.chainID, err)
			}
		})
	}
}

func TestValidate_MasterAddressOptional(t *testing.T) {
	// No master-address → valid (read-only profile).
	cfg := &Config{Profiles: map[string]Profile{
		"testnet": {RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11"},
	}}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("read-only profile should validate, got %v", err)
	}
}

func TestValidate_BypassRequiresMaster(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"p": {RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11", BypassHardLimits: true},
	}}
	if _, err := cfg.Validate(); err == nil {
		t.Fatal("bypass-hard-limits without master-address should be rejected")
	}
}

func TestValidate_BypassWithMasterAccepted(t *testing.T) {
	// bypass-hard-limits + a master-address is now valid. Under the old
	// allow-dangerous-tools model this required dangerous=true and would
	// have been rejected — this is the distinguishing behavior change.
	cfg := &Config{Profiles: map[string]Profile{
		"p": {
			RPCURL:           "https://rpc.test11.testnets.gno.land:443",
			ChainID:          "test11",
			BypassHardLimits: true,
			MasterAddress:    "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3",
		},
	}}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("bypass + master-address should be accepted, got %v", err)
	}
}

func TestValidate_DerivesChainType(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{
		"local":   {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"testnet": {RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11"},
	}}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got := cfg.Profiles["local"].ChainType; got != ChainTypeLocal {
		t.Errorf("local chain-type = %q, want %q", got, ChainTypeLocal)
	}
	if got := cfg.Profiles["testnet"].ChainType; got != ChainTypeTestnet {
		t.Errorf("testnet chain-type = %q, want %q", got, ChainTypeTestnet)
	}
}
