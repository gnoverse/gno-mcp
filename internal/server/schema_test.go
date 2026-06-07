// internal/server/schema_test.go
package server

import (
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

func TestProfileArgSchema_singleProfile_optionalDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	s := ProfileArgSchema(cfg, "" /* discovered */)
	if s.Required {
		t.Error("single profile: profile arg should be optional")
	}
	if s.Default != "testnet5" {
		t.Errorf("single profile: default should be the only profile, got %q", s.Default)
	}
}

func TestProfileArgSchema_multipleWithLocalDiscovered_optionalDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local":    {ChainType: "local", RPCURL: "x", ChainID: "dev"},
		"testnet5": {ChainType: "testnet", RPCURL: "y", ChainID: "test5"},
	}}
	s := ProfileArgSchema(cfg, "local")
	if s.Required {
		t.Error("local discovered: profile arg should be optional")
	}
	if s.Default != "local" {
		t.Errorf("local discovered: default should be 'local', got %q", s.Default)
	}
}

func TestProfileArgSchema_staleDiscoveredLocalIgnored(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
		"mainnet":  {ChainType: "mainnet", RPCURL: "y", ChainID: "portal-loop"},
	}}
	// Discovery returned a name that is not in the loaded config.
	// Stale name must not become the Default; falls back to smart default instead.
	s := ProfileArgSchema(cfg, "ghost")
	if s.Required {
		t.Error("stale discoveredLocal: profile arg should still be optional (smart default)")
	}
	if s.Default == "ghost" {
		t.Error("stale discoveredLocal must not become Default")
	}
	if s.Default == "" {
		t.Error("stale discoveredLocal: expected a smart default, got empty")
	}
}

func TestProfileArgSchema_multipleNoLocal_smartDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
		"mainnet":  {ChainType: "mainnet", RPCURL: "y", ChainID: "portal-loop"},
	}}
	// No local discovered and no "testnet" key — falls back to first alphabetical profile.
	s := ProfileArgSchema(cfg, "")
	if s.Required {
		t.Error("multi + no local: profile arg should be optional (smart default)")
	}
	if s.Default == "" {
		t.Error("multi + no local: expected a smart default, got empty")
	}
}

func TestProfileArgSchema_SmartDefault(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local":   {ChainID: "dev", ChainType: profiles.ChainTypeLocal, RPCURL: "x"},
		"testnet": {ChainID: "test11", ChainType: profiles.ChainTypeTestnet, RPCURL: "y"},
	}}
	// local discovered → default local
	if got := ProfileArgSchema(cfg, "local"); got.Default != "local" || got.Required {
		t.Errorf("with local discovered: default=%q required=%v", got.Default, got.Required)
	}
	// local NOT discovered → default testnet, still optional
	got := ProfileArgSchema(cfg, "")
	if got.Default != "testnet" || got.Required {
		t.Errorf("without local: default=%q required=%v, want testnet/false", got.Default, got.Required)
	}
}
