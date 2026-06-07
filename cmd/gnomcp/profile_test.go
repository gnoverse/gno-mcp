package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

func TestProfileAddManual_PersistsAndValidates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.toml")
	if err := profileAdd(path, "test11", profileAddOpts{
		RPC: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11",
	}); err != nil {
		t.Fatalf("add: %v", err)
	}
	f, _ := os.Open(path)
	defer f.Close()
	cfg, err := profiles.Load(f)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Profiles["test11"].RPCURL == "" {
		t.Error("profile not persisted")
	}
}

func TestProfileAdd_ReservedName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.toml")
	err := profileAdd(path, "local", profileAddOpts{RPC: "http://127.0.0.1:26657", ChainID: "dev"})
	if err == nil {
		t.Fatal("expected 'local' to be a reserved name")
	}
}

// TestParseProfileAddArgs_NameBeforeFlags regression-guards the bug where Go's
// flag parser stopped at the positional <name> and silently dropped every flag
// after it (so `add <name> --rpc ... --chain-id ...` lost rpc/chain-id).
func TestParseProfileAddArgs_NameBeforeFlags(t *testing.T) {
	name, o, err := parseProfileAddArgs([]string{"mychain", "--rpc", "https://rpc.test11.testnets.gno.land:443", "--chain-id", "test11"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if name != "mychain" {
		t.Errorf("name = %q, want mychain", name)
	}
	if o.RPC != "https://rpc.test11.testnets.gno.land:443" {
		t.Errorf("rpc = %q — flags after the name were dropped", o.RPC)
	}
	if o.ChainID != "test11" {
		t.Errorf("chain-id = %q, want test11", o.ChainID)
	}
}

func TestParseProfileAddArgs_FromGnowebAndMaster(t *testing.T) {
	name, o, err := parseProfileAddArgs([]string{"foo", "--from-gnoweb", "https://test11.testnets.gno.land", "--master", "g1abc"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if name != "foo" || o.FromGnoweb != "https://test11.testnets.gno.land" || o.Master != "g1abc" {
		t.Errorf("got name=%q from-gnoweb=%q master=%q", name, o.FromGnoweb, o.Master)
	}
}

func TestParseProfileAddArgs_MissingName(t *testing.T) {
	if _, _, err := parseProfileAddArgs([]string{"--rpc", "x"}); err == nil {
		t.Fatal("expected error when no name precedes the flags")
	}
}
