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
	s := ProfileArgSchema(cfg, "ghost")
	if !s.Required {
		t.Error("stale discoveredLocal should be ignored, falling back to required")
	}
	if s.Default != "" {
		t.Errorf("stale discoveredLocal should not become Default, got %q", s.Default)
	}
}

func TestProfileArgSchema_multipleNoLocal_required(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
		"mainnet":  {ChainType: "mainnet", RPCURL: "y", ChainID: "portal-loop"},
	}}
	s := ProfileArgSchema(cfg, "")
	if !s.Required {
		t.Error("multi + no local: profile arg should be required")
	}
	if s.Default != "" {
		t.Errorf("multi + no local: no default, got %q", s.Default)
	}
}
