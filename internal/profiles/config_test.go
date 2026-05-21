package profiles

import (
	"strings"
	"testing"
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
	if err != nil {
		t.Fatalf("Load returned err: %v", err)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}
	local, ok := cfg.Profiles["local"]
	if !ok {
		t.Fatal("missing local profile")
	}
	if local.ChainType != "local" || local.ChainID != "dev" {
		t.Errorf("local profile mis-parsed: %+v", local)
	}
	if local.RPCURL != "http://127.0.0.1:26657" {
		t.Errorf("local.RPCURL mis-parsed: got %q", local.RPCURL)
	}
	testnet := cfg.Profiles["testnet5"]
	if testnet.RPCURL != "https://rpc.test5.gno.land:443" {
		t.Errorf("testnet5.RPCURL mis-parsed: got %q", testnet.RPCURL)
	}
	if testnet.TxIndexerURL == "" {
		t.Error("testnet5 should have tx-indexer-url set")
	}
}

func TestLoad_malformedTOML(t *testing.T) {
	src := `[local
chain-type = "local"
`
	_, err := Load(strings.NewReader(src))
	if err == nil {
		t.Fatal("expected error for malformed TOML, got nil")
	}
}

func TestLoad_parsesAllowDangerousTools(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
allow-dangerous-tools = true
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Profiles["local"].AllowDangerousTools {
		t.Error("expected allow-dangerous-tools=true")
	}
}

func TestLoad_parsesDefaultSpendLimit(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-spend-limit = "1000000ugnot"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Profiles["local"].DefaultSpendLimit; got != "1000000ugnot" {
		t.Errorf("expected default-spend-limit=1000000ugnot, got %q", got)
	}
}

func TestLoad_parsesDefaultExpiresIn(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
default-expires-in = "4h"
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Profiles["local"].DefaultExpiresIn; got != "4h" {
		t.Errorf("expected default-expires-in=4h, got %q", got)
	}
}

func TestLoad_parsesBypassHardLimits(t *testing.T) {
	cfg, err := Load(strings.NewReader(`
[local]
rpc-url = "http://127.0.0.1:26657"
chain-id = "dev"
bypass-hard-limits = true
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Profiles["local"].BypassHardLimits {
		t.Error("expected bypass-hard-limits=true")
	}
}
