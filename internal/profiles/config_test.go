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
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := cfg.Profiles["local"]
	if p.MasterAddress != "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3" {
		t.Errorf("expected master-address=g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3, got %q", p.MasterAddress)
	}
	if got := p.DefaultSpendLimit; got != "1000000ugnot" {
		t.Errorf("expected default-spend-limit=1000000ugnot, got %q", got)
	}
	if got := p.DefaultExpiresIn; got != "4h" {
		t.Errorf("expected default-expires-in=4h, got %q", got)
	}
	if !p.BypassHardLimits {
		t.Error("expected bypass-hard-limits=true")
	}
}
